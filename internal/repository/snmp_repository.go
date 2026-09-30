package repository

import (
	"context"
	"fmt"
	"strings"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/database"
)

type SnmpRepository struct{}

func NewSnmpRepository() *SnmpRepository {
	return &SnmpRepository{}
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

	// Strictly deduplicate by OID to prevent PostgreSQL SQLSTATE 21000 (ON CONFLICT DO UPDATE cannot affect row a second time)
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

	batchSize := 100
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
	return nil
}

func (r *SnmpRepository) TranslateOid(ctx context.Context, numericOid string) (string, *domain.OidRegistry) {
	cleanOid := strings.Trim(numericOid, ".")
	pool, err := database.GetPool()
	if err != nil {
		return cleanOid, nil
	}

	query := `SELECT oid, name, mib_name, syntax, access, description, created_at
              FROM oid_registry
              WHERE $1 LIKE oid || '%'
              ORDER BY length(oid) DESC LIMIT 1`

	var o domain.OidRegistry
	row := pool.QueryRow(ctx, query, cleanOid)
	if err := row.Scan(&o.OID, &o.Name, &o.MibName, &o.Syntax, &o.Access, &o.Description, &o.CreatedAt); err == nil {
		suffix := strings.TrimPrefix(cleanOid, o.OID)
		displayName := o.Name
		if suffix != "" {
			if strings.HasPrefix(suffix, ".") {
				displayName = o.Name + suffix
			} else {
				displayName = o.Name + "." + suffix
			}
		}
		return displayName, &o
	}

	// Standard and vendor MIB prefix translations ordered from longest to shortest
	type prefixEntry struct {
		prefix string
		name   string
	}
	standardPrefixes := []prefixEntry{
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
		{"1.3.6.1.2.1.2.2.1.2", "ifDescr"},
		{"1.3.6.1.2.1.2.2.1.3", "ifType"},
		{"1.3.6.1.2.1.2.2.1.5", "ifSpeed"},
		{"1.3.6.1.2.1.2.2.1.6", "ifPhysAddress"},
		{"1.3.6.1.2.1.2.2.1.9", "ifLastChange"},

		// HOST-RESOURCES-MIB (RFC 2790)
		{"1.3.6.1.2.1.25.3.3.1.2", "hrProcessorLoad"},
		{"1.3.6.1.2.1.25.3.3.1.1", "hrProcessorFrwID"},
		{"1.3.6.1.2.1.25.2.3.1.6", "hrStorageUsed"},
		{"1.3.6.1.2.1.25.2.3.1.5", "hrStorageSize"},
		{"1.3.6.1.2.1.25.2.3.1.4", "hrStorageAllocationUnits"},
		{"1.3.6.1.2.1.25.2.3.1.3", "hrStorageDescr"},
		{"1.3.6.1.2.1.25.2.3.1.2", "hrStorageType"},
		{"1.3.6.1.2.1.25.2.3.1.1", "hrStorageIndex"},
		{"1.3.6.1.2.1.25.2.2", "hrMemorySize"},
		{"1.3.6.1.2.1.25.1.1", "hrSystemUptime"},

		// SNMPv2-MIB / System (RFC 3418)
		{"1.3.6.1.2.1.1.1", "sysDescr"},
		{"1.3.6.1.2.1.1.2", "sysObjectID"},
		{"1.3.6.1.2.1.1.3", "sysUpTime"},
		{"1.3.6.1.2.1.1.4", "sysContact"},
		{"1.3.6.1.2.1.1.5", "sysName"},
		{"1.3.6.1.2.1.1.6", "sysLocation"},

		// UCD-SNMP-MIB (Linux)
		{"1.3.6.1.4.1.2021.10.1.3", "laLoad"},
		{"1.3.6.1.4.1.2021.4.5", "memTotalReal"},
		{"1.3.6.1.4.1.2021.4.6", "memAvailReal"},
		{"1.3.6.1.4.1.2021.4.11", "memTotalFree"},
		{"1.3.6.1.4.1.2021.4.14", "memBuffer"},
		{"1.3.6.1.4.1.2021.4.15", "memCached"},

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

		// Cisco MIBs
		{"1.3.6.1.4.1.9.9.109.1.1.1.1.8", "cpmCPUTotal5minRev"},
		{"1.3.6.1.4.1.9.9.109.1.1.1.1.5", "cpmCPUTotal5min"},
		{"1.3.6.1.4.1.9.9.109.1.1.1.1.4", "cpmCPUTotal1min"},
		{"1.3.6.1.4.1.9.9.109.1.1.1.1.3", "cpmCPUTotal5sec"},
		{"1.3.6.1.4.1.9.9.48.1.1.1.5", "ciscoMemoryPoolUsed"},
		{"1.3.6.1.4.1.9.9.48.1.1.1.6", "ciscoMemoryPoolFree"},
		{"1.3.6.1.4.1.9.9.13.1.3.1.3", "ciscoEnvMonTemperatureValue"},
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

		// Broad fallbacks
		{"1.3.6.1.2.1.1", "system"},
		{"1.3.6.1.2.1.2", "interfaces"},
		{"1.3.6.1.2.1.4", "ip"},
		{"1.3.6.1.2.1.5", "icmp"},
		{"1.3.6.1.2.1.6", "tcp"},
		{"1.3.6.1.2.1.7", "udp"},
		{"1.3.6.1.2.1.25", "hostResources"},
		{"1.3.6.1.4.1", "enterprises"},
	}

	for _, entry := range standardPrefixes {
		if strings.HasPrefix(cleanOid, entry.prefix) {
			suffix := strings.TrimPrefix(cleanOid, entry.prefix)
			if suffix == "" {
				return entry.name, nil
			}
			if strings.HasPrefix(suffix, ".") {
				return entry.name + suffix, nil
			}
			return entry.name + "." + suffix, nil
		}
	}

	return cleanOid, nil
}
