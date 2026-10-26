package repository

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/database"
)

type cachedOidEntry struct {
	oid         string
	name        string
	mibName     string
	syntax      *string
	access      *string
	description *string
}

type prefixEntry struct {
	prefix string
	name   string
}

type SnmpRepository struct {
	cacheMu      sync.RWMutex
	cacheLoaded  bool
	exactOids    map[string]*domain.OidRegistry
	sortedPrefix []cachedOidEntry
	memoNames    map[string]string
}

func NewSnmpRepository() *SnmpRepository {
	return &SnmpRepository{
		exactOids: make(map[string]*domain.OidRegistry),
		memoNames: make(map[string]string),
	}
}

func (r *SnmpRepository) InvalidateCache() {
	r.cacheMu.Lock()
	defer r.cacheMu.Unlock()
	r.cacheLoaded = false
	r.exactOids = make(map[string]*domain.OidRegistry)
	r.sortedPrefix = nil
	r.memoNames = make(map[string]string)
}

func (r *SnmpRepository) loadCache(ctx context.Context) {
	pool, err := database.GetPool()
	if err != nil {
		return
	}

	rows, err := pool.Query(ctx, `SELECT oid, name, mib_name, syntax, access, description, created_at FROM oid_registry`)
	if err != nil {
		return
	}
	defer rows.Close()

	exacts := make(map[string]*domain.OidRegistry)
	var prefixes []cachedOidEntry

	for rows.Next() {
		var o domain.OidRegistry
		if err := rows.Scan(&o.OID, &o.Name, &o.MibName, &o.Syntax, &o.Access, &o.Description, &o.CreatedAt); err == nil {
			clean := strings.Trim(strings.TrimSpace(o.OID), ".")
			if clean == "" {
				continue
			}
			o.OID = clean
			exacts[clean] = &o
			prefixes = append(prefixes, cachedOidEntry{
				oid:         clean,
				name:        o.Name,
				mibName:     o.MibName,
				syntax:      o.Syntax,
				access:      o.Access,
				description: o.Description,
			})
		}
	}

	// Sort prefixes descending by OID length so longest/most specific match wins
	sort.Slice(prefixes, func(i, j int) bool {
		if len(prefixes[i].oid) != len(prefixes[j].oid) {
			return len(prefixes[i].oid) > len(prefixes[j].oid)
		}
		return prefixes[i].oid > prefixes[j].oid
	})

	r.cacheMu.Lock()
	r.exactOids = exacts
	r.sortedPrefix = prefixes
	r.memoNames = make(map[string]string)
	r.cacheLoaded = true
	r.cacheMu.Unlock()
}

func (r *SnmpRepository) ListImportedMibs(ctx context.Context) ([]domain.ImportedMib, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, `SELECT name, node_count, imported_at FROM imported_mibs ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.ImportedMib
	for rows.Next() {
		var m domain.ImportedMib
		if err := rows.Scan(&m.Name, &m.NodeCount, &m.ImportedAt); err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, nil
}

func (r *SnmpRepository) SaveImportedMib(ctx context.Context, name string, nodeCount int) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `INSERT INTO imported_mibs (name, node_count, imported_at) VALUES ($1, $2, NOW())
                             ON CONFLICT (name) DO UPDATE SET node_count = EXCLUDED.node_count, imported_at = NOW()`,
		name, nodeCount)
	return err
}

func (r *SnmpRepository) DeleteImportedMib(ctx context.Context, name string) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}
	// ON DELETE CASCADE automatically deletes corresponding entries from oid_registry
	_, err = pool.Exec(ctx, `DELETE FROM imported_mibs WHERE name = $1`, name)
	if err == nil {
		r.InvalidateCache()
	}
	return err
}

func (r *SnmpRepository) SaveOidBatch(ctx context.Context, oids []domain.OidRegistry) error {
	if len(oids) == 0 {
		return nil
	}

	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	// Strictly deduplicate by OID to prevent PostgreSQL SQLSTATE 21000
	dedupMap := make(map[string]domain.OidRegistry)
	for _, o := range oids {
		cleanOid := strings.Trim(strings.TrimSpace(o.OID), ".")
		if cleanOid != "" {
			o.OID = cleanOid
			dedupMap[cleanOid] = o
		}
	}

	deduped := make([]domain.OidRegistry, 0, len(dedupMap))
	for _, o := range dedupMap {
		deduped = append(deduped, o)
	}

	batchSize := 500
	for i := 0; i < len(deduped); i += batchSize {
		end := i + batchSize
		if end > len(deduped) {
			end = len(deduped)
		}
		chunk := deduped[i:end]

		var valueStrings []string
		var valueArgs []interface{}
		for idx, o := range chunk {
			pos := idx*6 + 1
			valueStrings = append(valueStrings, fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d)", pos, pos+1, pos+2, pos+3, pos+4, pos+5))
			valueArgs = append(valueArgs, o.OID, o.Name, o.MibName, o.Syntax, o.Access, o.Description)
		}

		query := fmt.Sprintf(`INSERT INTO oid_registry (oid, name, mib_name, syntax, access, description)
                              VALUES %s ON CONFLICT (oid) DO UPDATE SET
                                name = EXCLUDED.name, mib_name = EXCLUDED.mib_name, syntax = EXCLUDED.syntax,
                                access = EXCLUDED.access, description = EXCLUDED.description`, strings.Join(valueStrings, ", "))
		_, err := pool.Exec(ctx, query, valueArgs...)
		if err != nil {
			return err
		}
	}

	r.InvalidateCache()
	return nil
}

func (r *SnmpRepository) TranslateOid(ctx context.Context, numericOid string) (string, *domain.OidRegistry) {
	cleanOid := strings.Trim(strings.TrimSpace(numericOid), ".")
	if cleanOid == "" {
		return "", nil
	}

	// 1. Fast path: check in-memory cached translations
	r.cacheMu.RLock()
	if r.cacheLoaded {
		if memo, ok := r.memoNames[cleanOid]; ok {
			exact := r.exactOids[cleanOid]
			r.cacheMu.RUnlock()
			return memo, exact
		}
		if exact, ok := r.exactOids[cleanOid]; ok {
			r.cacheMu.RUnlock()
			return exact.Name, exact
		}
	}
	r.cacheMu.RUnlock()

	// 2. If cache not loaded yet, lazily load database OID definitions
	r.cacheMu.RLock()
	loaded := r.cacheLoaded
	r.cacheMu.RUnlock()
	if !loaded {
		r.loadCache(ctx)
	}

	// 3. Search loaded MIB definitions in memory (longest prefix with strict dot boundary)
	r.cacheMu.RLock()
	if r.cacheLoaded {
		// Exact match
		if exact, ok := r.exactOids[cleanOid]; ok {
			r.cacheMu.RUnlock()
			r.recordMemo(cleanOid, exact.Name)
			return exact.Name, exact
		}

		// Longest parent prefix match (strict numeric dot boundary)
		for _, entry := range r.sortedPrefix {
			if strings.HasPrefix(cleanOid, entry.oid+".") {
				suffix := strings.TrimPrefix(cleanOid, entry.oid+".")
				displayName := entry.name + "." + suffix
				reg := &domain.OidRegistry{
					OID:         entry.oid,
					Name:        entry.name,
					MibName:     entry.mibName,
					Syntax:      entry.syntax,
					Access:      entry.access,
					Description: entry.description,
				}
				r.cacheMu.RUnlock()
				r.recordMemo(cleanOid, displayName)
				return displayName, reg
			}
		}
	}
	r.cacheMu.RUnlock()

	// 4. Fallback: Direct PostgreSQL query with strict dot-boundary condition
	pool, poolErr := database.GetPool()
	if poolErr == nil {
		query := `SELECT oid, name, mib_name, syntax, access, description, created_at
	              FROM oid_registry
	              WHERE oid = $1 OR $1 LIKE oid || '.%'
	              ORDER BY length(oid) DESC LIMIT 1`

		var o domain.OidRegistry
		row := pool.QueryRow(ctx, query, cleanOid)
		if err := row.Scan(&o.OID, &o.Name, &o.MibName, &o.Syntax, &o.Access, &o.Description, &o.CreatedAt); err == nil {
			var displayName string
			if cleanOid == o.OID {
				displayName = o.Name
			} else if strings.HasPrefix(cleanOid, o.OID+".") {
				suffix := strings.TrimPrefix(cleanOid, o.OID+".")
				displayName = o.Name + "." + suffix
			} else {
				displayName = o.Name
			}
			r.recordMemo(cleanOid, displayName)
			return displayName, &o
		}
	}

	// 5. Standard and vendor MIB prefix translations (ordered longest to shortest with strict dot boundaries)
	for _, entry := range standardPrefixes {
		if cleanOid == entry.prefix {
			r.recordMemo(cleanOid, entry.name)
			return entry.name, nil
		}
		if strings.HasPrefix(cleanOid, entry.prefix+".") {
			suffix := strings.TrimPrefix(cleanOid, entry.prefix+".")
			displayName := entry.name + "." + suffix
			r.recordMemo(cleanOid, displayName)
			return displayName, nil
		}
	}

	r.recordMemo(cleanOid, cleanOid)
	return cleanOid, nil
}

func (r *SnmpRepository) recordMemo(cleanOid, name string) {
	r.cacheMu.Lock()
	if r.memoNames != nil {
		r.memoNames[cleanOid] = name
	}
	r.cacheMu.Unlock()
}

// standardPrefixes contains standard RFC and vendor MIB symbols ordered from longest to shortest
var standardPrefixes = []prefixEntry{
	// IF-MIB (RFC 2863)
	{"1.3.6.1.2.1.31.1.1.1.15", "ifHighSpeed"},
	{"1.3.6.1.2.1.31.1.1.1.18", "ifAlias"},
	{"1.3.6.1.2.1.31.1.1.1.1", "ifName"},
	{"1.3.6.1.2.1.31.1.1.1.6", "ifHCInOctets"},
	{"1.3.6.1.2.1.31.1.1.1.7", "ifHCInUcastPkts"},
	{"1.3.6.1.2.1.31.1.1.1.8", "ifHCInMulticastPkts"},
	{"1.3.6.1.2.1.31.1.1.1.9", "ifHCInBroadcastPkts"},
	{"1.3.6.1.2.1.31.1.1.1.10", "ifHCOutOctets"},
	{"1.3.6.1.2.1.31.1.1.1.11", "ifHCOutUcastPkts"},
	{"1.3.6.1.2.1.31.1.1.1.12", "ifHCOutMulticastPkts"},
	{"1.3.6.1.2.1.31.1.1.1.13", "ifHCOutBroadcastPkts"},
	{"1.3.6.1.2.1.2.2.1.8", "ifOperStatus"},
	{"1.3.6.1.2.1.2.2.1.7", "ifAdminStatus"},
	{"1.3.6.1.2.1.2.2.1.10", "ifInOctets"},
	{"1.3.6.1.2.1.2.2.1.16", "ifOutOctets"},
	{"1.3.6.1.2.1.2.2.1.14", "ifInErrors"},
	{"1.3.6.1.2.1.2.2.1.20", "ifOutErrors"},
	{"1.3.6.1.2.1.2.2.1.13", "ifInDiscards"},
	{"1.3.6.1.2.1.2.2.1.19", "ifOutDiscards"},
	{"1.3.6.1.2.1.2.2.1.11", "ifInUcastPkts"},
	{"1.3.6.1.2.1.2.2.1.17", "ifOutUcastPkts"},
	{"1.3.6.1.2.1.2.2.1.2", "ifDescr"},
	{"1.3.6.1.2.1.2.2.1.3", "ifType"},
	{"1.3.6.1.2.1.2.2.1.5", "ifSpeed"},
	{"1.3.6.1.2.1.2.2.1.6", "ifPhysAddress"},
	{"1.3.6.1.2.1.2.2.1.9", "ifLastChange"},
	{"1.3.6.1.2.1.2.2.1.1", "ifIndex"},
	{"1.3.6.1.2.1.2.1", "ifNumber"},

	// IP-MIB (RFC 2011/4293)
	{"1.3.6.1.2.1.4.20.1.1", "ipAdEntAddr"},
	{"1.3.6.1.2.1.4.20.1.2", "ipAdEntIfIndex"},
	{"1.3.6.1.2.1.4.20.1.3", "ipAdEntNetMask"},
	{"1.3.6.1.2.1.4.20.1.4", "ipAdEntBcastAddr"},
	{"1.3.6.1.2.1.4.20.1.5", "ipAdEntReasmMaxSize"},
	{"1.3.6.1.2.1.4.22.1.1", "ipNetToMediaIfIndex"},
	{"1.3.6.1.2.1.4.22.1.2", "ipNetToMediaPhysAddress"},
	{"1.3.6.1.2.1.4.22.1.3", "ipNetToMediaNetAddress"},
	{"1.3.6.1.2.1.4.22.1.4", "ipNetToMediaType"},
	{"1.3.6.1.2.1.4.21.1.1", "ipRouteDest"},
	{"1.3.6.1.2.1.4.21.1.2", "ipRouteIfIndex"},
	{"1.3.6.1.2.1.4.21.1.7", "ipRouteNextHop"},
	{"1.3.6.1.2.1.4.21.1.11", "ipRouteMask"},
	{"1.3.6.1.2.1.4.1", "ipForwarding"},
	{"1.3.6.1.2.1.4.2", "ipDefaultTTL"},
	{"1.3.6.1.2.1.4.3", "ipInReceives"},

	// HOST-RESOURCES-MIB (RFC 2790)
	{"1.3.6.1.2.1.25.3.3.1.2", "hrProcessorLoad"},
	{"1.3.6.1.2.1.25.3.3.1.1", "hrProcessorFrwID"},
	{"1.3.6.1.2.1.25.3.2.1.5", "hrDeviceStatus"},
	{"1.3.6.1.2.1.25.3.2.1.3", "hrDeviceDescr"},
	{"1.3.6.1.2.1.25.3.2.1.2", "hrDeviceType"},
	{"1.3.6.1.2.1.25.3.2.1.1", "hrDeviceIndex"},
	{"1.3.6.1.2.1.25.2.3.1.6", "hrStorageUsed"},
	{"1.3.6.1.2.1.25.2.3.1.5", "hrStorageSize"},
	{"1.3.6.1.2.1.25.2.3.1.4", "hrStorageAllocationUnits"},
	{"1.3.6.1.2.1.25.2.3.1.3", "hrStorageDescr"},
	{"1.3.6.1.2.1.25.2.3.1.2", "hrStorageType"},
	{"1.3.6.1.2.1.25.2.3.1.1", "hrStorageIndex"},
	{"1.3.6.1.2.1.25.4.2.1.2", "hrSWRunName"},
	{"1.3.6.1.2.1.25.4.2.1.7", "hrSWRunStatus"},
	{"1.3.6.1.2.1.25.2.2", "hrMemorySize"},
	{"1.3.6.1.2.1.25.1.1", "hrSystemUptime"},
	{"1.3.6.1.2.1.25.1.2", "hrSystemDate"},

	// SNMPv2-MIB / System (RFC 3418)
	{"1.3.6.1.2.1.1.1", "sysDescr"},
	{"1.3.6.1.2.1.1.2", "sysObjectID"},
	{"1.3.6.1.2.1.1.3", "sysUpTime"},
	{"1.3.6.1.2.1.1.4", "sysContact"},
	{"1.3.6.1.2.1.1.5", "sysName"},
	{"1.3.6.1.2.1.1.6", "sysLocation"},
	{"1.3.6.1.2.1.1.7", "sysServices"},

	// TCP & UDP MIBs
	{"1.3.6.1.2.1.6.13.1.1", "tcpConnState"},
	{"1.3.6.1.2.1.6.13.1.2", "tcpConnLocalAddress"},
	{"1.3.6.1.2.1.6.13.1.3", "tcpConnLocalPort"},
	{"1.3.6.1.2.1.7.5.1.1", "udpLocalAddress"},
	{"1.3.6.1.2.1.7.5.1.2", "udpLocalPort"},

	// PRINTER-MIB (RFC 3805)
	{"1.3.6.1.2.1.43.11.1.1.9", "prtMarkerSuppliesLevel"},
	{"1.3.6.1.2.1.43.11.1.1.8", "prtMarkerSuppliesMaxCapacity"},
	{"1.3.6.1.2.1.43.11.1.1.6", "prtMarkerSuppliesDescription"},
	{"1.3.6.1.2.1.43.11.1.1.5", "prtMarkerSuppliesType"},
	{"1.3.6.1.2.1.43.10.2.1.4", "prtMarkerLifeCount"},

	// ENTITY-MIB (RFC 4133)
	{"1.3.6.1.2.1.47.1.1.1.1.7", "entPhysicalName"},
	{"1.3.6.1.2.1.47.1.1.1.1.2", "entPhysicalDescr"},
	{"1.3.6.1.2.1.47.1.1.1.1.5", "entPhysicalClass"},
	{"1.3.6.1.2.1.47.1.1.1.1.11", "entPhysicalSerialNum"},

	// LLDP-MIB (IEEE 802.1AB)
	{"1.0.8802.1.1.2.1.4.1.1.9", "lldpRemSysName"},
	{"1.0.8802.1.1.2.1.4.1.1.8", "lldpRemPortDesc"},
	{"1.0.8802.1.1.2.1.4.1.1.7", "lldpRemPortId"},

	// UCD-SNMP-MIB (Linux)
	{"1.3.6.1.4.1.2021.10.1.3", "laLoad"},
	{"1.3.6.1.4.1.2021.4.5", "memTotalReal"},
	{"1.3.6.1.4.1.2021.4.6", "memAvailReal"},
	{"1.3.6.1.4.1.2021.4.11", "memTotalFree"},
	{"1.3.6.1.4.1.2021.4.14", "memBuffer"},
	{"1.3.6.1.4.1.2021.4.15", "memCached"},
	{"1.3.6.1.4.1.2021.11.9", "ssCpuUser"},
	{"1.3.6.1.4.1.2021.11.10", "ssCpuSystem"},
	{"1.3.6.1.4.1.2021.11.11", "ssCpuIdle"},

	// MikroTik MIB
	{"1.3.6.1.4.1.14988.1.1.1.2.1.1", "mtxrOpticalRxPower"},
	{"1.3.6.1.4.1.14988.1.1.1.2.1.2", "mtxrOpticalTxPower"},
	{"1.3.6.1.4.1.14988.1.1.1.2.1.3", "mtxrOpticalTemp"},
	{"1.3.6.1.4.1.14988.1.1.1.2.1.4", "mtxrOpticalVoltage"},
	{"1.3.6.1.4.1.14988.1.1.1.2.1.5", "mtxrOpticalBiasCurrent"},
	{"1.3.6.1.4.1.14988.1.1.3.10", "mtxrHealthTemperature"},
	{"1.3.6.1.4.1.14988.1.1.3.11", "mtxrHealthProcessorTemperature"},
	{"1.3.6.1.4.1.14988.1.1.3.8", "mtxrHealthVoltage"},
	{"1.3.6.1.4.1.14988.1.1.3.14", "mtxrHealthCurrent"},
	{"1.3.6.1.4.1.14988.1.1.3.12", "mtxrHealthPowerConsumption"},

	// Cisco MIBs
	{"1.3.6.1.4.1.9.9.109.1.1.1.1.8", "cpmCPUTotal5minRev"},
	{"1.3.6.1.4.1.9.9.109.1.1.1.1.5", "cpmCPUTotal5min"},
	{"1.3.6.1.4.1.9.9.109.1.1.1.1.4", "cpmCPUTotal1min"},
	{"1.3.6.1.4.1.9.9.109.1.1.1.1.3", "cpmCPUTotal5sec"},
	{"1.3.6.1.4.1.9.9.48.1.1.1.5", "ciscoMemoryPoolUsed"},
	{"1.3.6.1.4.1.9.9.48.1.1.1.6", "ciscoMemoryPoolFree"},
	{"1.3.6.1.4.1.9.9.13.1.3.1.3", "ciscoEnvMonTemperatureValue"},
	{"1.3.6.1.4.1.9.9.13.1.4.1.3", "ciscoEnvMonFanStatus"},
	{"1.3.6.1.4.1.9.9.91.1.1.1.1.4", "entSensorValue"},

	// Huawei MIBs
	{"1.3.6.1.4.1.2011.6.3.4.1.2", "hwEntityOpticalRxPower"},
	{"1.3.6.1.4.1.2011.6.3.4.1.3", "hwEntityOpticalTxPower"},
	{"1.3.6.1.4.1.2011.6.3.4.1.4", "hwEntityOpticalTemp"},
	{"1.3.6.1.4.1.2011.6.3.4.1.5", "hwEntityOpticalVoltage"},
	{"1.3.6.1.4.1.2011.6.3.4.1.6", "hwEntityOpticalBiasCurrent"},
	{"1.3.6.1.4.1.2011.6.128.1.1.2.23.1.4", "hwGponOntRxPower"},
	{"1.3.6.1.4.1.2011.6.128.1.1.2.23.1.5", "hwGponOntTxPower"},
	{"1.3.6.1.4.1.2011.6.128.1.1.2.23.1.6", "hwGponOntVoltage"},
	{"1.3.6.1.4.1.2011.6.128.1.1.2.23.1.7", "hwGponOntBiasCurrent"},
	{"1.3.6.1.4.1.2011.6.128.1.1.2.23.1.8", "hwGponOntTemperature"},
	{"1.3.6.1.4.1.2011.6.128.1.1.2.23.1.9", "hwGponOntOLTRxPower"},
	{"1.3.6.1.4.1.2011.5.25.31.1.1.1.1.5", "hwEntityCpuUsage"},
	{"1.3.6.1.4.1.2011.5.25.31.1.1.1.1.7", "hwEntityMemUsage"},
	{"1.3.6.1.4.1.2011.5.25.31.1.1.1.1.11", "hwEntityTemperature"},

	// ZTE GPON
	{"1.3.6.1.4.1.3902.1082.500.1.2.3.1.2", "zteGponOntRxPower"},
	{"1.3.6.1.4.1.3902.1082.500.1.2.3.1.3", "zteGponOntTxPower"},
	{"1.3.6.1.4.1.3902.1082.500.1.2.3.1.4", "zteGponOntVoltage"},
	{"1.3.6.1.4.1.3902.1082.500.1.2.3.1.5", "zteGponOntCurrent"},
	{"1.3.6.1.4.1.3902.1082.500.1.2.3.1.6", "zteGponOntTemperature"},
	{"1.3.6.1.4.1.3902.1082.500.1.2.3.1.7", "zteGponOntOLTRxPower"},

	// Broad subtrees
	{"1.3.6.1.2.1.43", "printer"},
	{"1.3.6.1.2.1.25", "hostResources"},
	{"1.3.6.1.2.1.31", "ifMIB"},
	{"1.3.6.1.2.1.47", "entityMIB"},
	{"1.3.6.1.2.1.17", "dot1dBridge"},
	{"1.3.6.1.2.1.16", "rmon"},
	{"1.3.6.1.2.1.10", "transmission"},
	{"1.3.6.1.2.1.7", "udp"},
	{"1.3.6.1.2.1.6", "tcp"},
	{"1.3.6.1.2.1.5", "icmp"},
	{"1.3.6.1.2.1.4", "ip"},
	{"1.3.6.1.2.1.2", "interfaces"},
	{"1.3.6.1.2.1.1", "system"},
	{"1.3.6.1.4.1.9", "cisco"},
	{"1.3.6.1.4.1.2011", "huawei"},
	{"1.3.6.1.4.1.3902", "zte"},
	{"1.3.6.1.4.1.14988", "mikrotik"},
	{"1.3.6.1.4.1.6486", "alcatel"},
	{"1.3.6.1.4.1.8072", "netSnmp"},
	{"1.3.6.1.4.1.2021", "ucdavis"},
	{"1.3.6.1.4.1", "enterprises"},
	{"1.3.6.1.2.1", "mib-2"},
	{"1.3.6.1.2", "mgmt"},
	{"1.3.6.1", "internet"},
}

