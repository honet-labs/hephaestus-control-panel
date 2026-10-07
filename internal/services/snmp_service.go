package services

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/logger"
	"go-hephaestus/internal/repository"

	"github.com/gosnmp/gosnmp"
)

type SnmpService struct {
	snmpRepo        *repository.SnmpRepository
	mibsDir         string
	discoveryEngine *SnmpDiscoveryEngine
	subnetScanner   *SnmpSubnetScanner
}

func NewSnmpService(snmpRepo *repository.SnmpRepository, dataDir string) *SnmpService {
	mibsDir := filepath.Join(dataDir, "mibs")
	_ = os.MkdirAll(mibsDir, 0755)

	s := &SnmpService{
		snmpRepo: snmpRepo,
		mibsDir:  mibsDir,
	}
	s.discoveryEngine = NewSnmpDiscoveryEngine(s)
	s.subnetScanner = NewSnmpSubnetScanner()

	go s.SyncMibsFromDisk(context.Background())
	return s
}

func (s *SnmpService) DiscoverDevice(host string, port uint16, version string, community string, profile string, timeoutSec int, retries int) (*domain.SnmpDiscoveryResult, error) {
	return s.discoveryEngine.Discover(host, port, version, community, profile, timeoutSec, retries)
}

func (s *SnmpService) ScanSubnet(req domain.SnmpSubnetScanRequest) (*domain.SnmpSubnetScanResult, error) {
	return s.subnetScanner.Scan(req)
}

func (s *SnmpService) GetProfiles() []domain.SnmpDiscoveryProfileInfo {
	return AvailableProfiles()
}

func normalizeOid(rawOid string, operation string) (string, string) {
	trimmed := strings.TrimSpace(rawOid)
	op := strings.ToLower(strings.TrimSpace(operation))
	if op == "" {
		op = "walk"
	}

	// Direct wildcard aliases for scanning the entire MIB tree
	if trimmed == "" || trimmed == "*" || trimmed == ".*" || strings.EqualFold(trimmed, "all") || strings.EqualFold(trimmed, "root") {
		return "1.3.6.1", "walk"
	}

	// Check standard textual MIB names
	standardNames := map[string]string{
		"system":      "1.3.6.1.2.1.1",
		"interfaces":  "1.3.6.1.2.1.2",
		"ip":          "1.3.6.1.2.1.4",
		"icmp":        "1.3.6.1.2.1.5",
		"tcp":         "1.3.6.1.2.1.6",
		"udp":         "1.3.6.1.2.1.7",
		"snmp":        "1.3.6.1.2.1.11",
		"host":        "1.3.6.1.2.1.25",
		"enterprises": "1.3.6.1.4.1",
		"mgmt":        "1.3.6.1.2",
		"mib-2":       "1.3.6.1.2.1",
		"internet":    "1.3.6.1",
	}
	if mapped, exists := standardNames[strings.ToLower(trimmed)]; exists {
		return mapped, op
	}

	// Strip trailing wildcard characters: e.g. "1.3.6.1.2.1.*" or "1.3.6.1.2.1*" -> "1.3.6.1.2.1"
	trimmed = strings.TrimSuffix(trimmed, "*")
	trimmed = strings.Trim(trimmed, ".")

	if trimmed == "" || trimmed == "1.3.6.1" || trimmed == "1.3" || trimmed == "1" {
		return "1.3.6.1", "walk"
	}

	return trimmed, op
}

func (s *SnmpService) Query(host string, port uint16, version string, community string, startOid string, operation string, timeoutSec int, retries int) ([]domain.SnmpQueryResult, error) {
	if port == 0 {
		port = 161
	}
	if community == "" {
		community = "public"
	}
	if timeoutSec <= 0 {
		timeoutSec = 6
	}
	if retries <= 0 {
		retries = 3
	}

	cleanOid, operation := normalizeOid(startOid, operation)
	if !strings.HasPrefix(cleanOid, ".") {
		cleanOid = "." + cleanOid
	}

	snmpVersion := gosnmp.Version2c
	if version == "v1" {
		snmpVersion = gosnmp.Version1
	}

	params := &gosnmp.GoSNMP{
		Target:             host,
		Port:               port,
		Transport:          "udp",
		Community:          community,
		Version:            snmpVersion,
		Timeout:            time.Duration(timeoutSec) * time.Second,
		Retries:            retries,
		ExponentialTimeout: true,
		MaxRepetitions:     50,
		MaxOids:            gosnmp.Default.MaxOids,
	}

	if err := params.Connect(); err != nil {
		return nil, fmt.Errorf("SNMP connect failed to %s:%d: %w", host, port, err)
	}
	defer params.Conn.Close()

	var results []domain.SnmpQueryResult
	ctx := context.Background()

	if operation == "get" {
		pkt, err := params.Get([]string{cleanOid})
		if err != nil {
			return nil, s.enrichSnmpError(err, host, port, community)
		}
		for _, v := range pkt.Variables {
			if v.Type != gosnmp.NoSuchObject && v.Type != gosnmp.NoSuchInstance && v.Type != gosnmp.Null {
				oidStr := strings.TrimPrefix(v.Name, ".")
				name, _ := s.snmpRepo.TranslateOid(ctx, oidStr)
				valStr, typeStr := formatVarbind(v)
				results = append(results, domain.SnmpQueryResult{
					OID:   oidStr,
					Name:  name,
					Value: valStr,
					Type:  typeStr,
				})
			}
		}
	} else {
		// Walk operation: supports standard subtree and scalar leaves (.0)
		isScalar := strings.HasSuffix(cleanOid, ".0")
		const maxWalkResults = 3000
		var limitReached bool

		walkFn := func(dataUnit gosnmp.SnmpPDU) error {
			if len(results) >= maxWalkResults {
				limitReached = true
				return fmt.Errorf("walk_limit_reached")
			}
			oidStr := strings.TrimPrefix(dataUnit.Name, ".")
			name, _ := s.snmpRepo.TranslateOid(ctx, oidStr)
			valStr, typeStr := formatVarbind(dataUnit)
			results = append(results, domain.SnmpQueryResult{
				OID:   oidStr,
				Name:  name,
				Value: valStr,
				Type:  typeStr,
			})
			return nil
		}

		var walkErr error
		if snmpVersion == gosnmp.Version2c {
			walkErr = params.BulkWalk(cleanOid, walkFn)
			// Fallback: If BulkWalk failed or returned 0 results, try standard Walk (GETNEXT)
			if (walkErr != nil || len(results) == 0) && !limitReached {
				walkErr = params.Walk(cleanOid, walkFn)
			}
		} else {
			walkErr = params.Walk(cleanOid, walkFn)
		}

		// Fallback for full root walk (1.3.6.1): if 0 results, try standard MIB-2 (.1.3.6.1.2.1)
		if (cleanOid == ".1.3.6.1" || cleanOid == ".1.3") && len(results) == 0 && !limitReached {
			if snmpVersion == gosnmp.Version2c {
				_ = params.BulkWalk(".1.3.6.1.2.1", walkFn)
			}
			if len(results) == 0 {
				_ = params.Walk(".1.3.6.1.2.1", walkFn)
			}
		}

		// If Walk returned 0 results or failed, and user specified a scalar OID (e.g. .1.3.6.1.2.1.1.1.0)
		if isScalar && len(results) == 0 {
			// 1. Try direct GET on the exact scalar OID
			if pkt, getErr := params.Get([]string{cleanOid}); getErr == nil && len(pkt.Variables) > 0 {
				for _, v := range pkt.Variables {
					if v.Type != gosnmp.NoSuchObject && v.Type != gosnmp.NoSuchInstance && v.Type != gosnmp.Null {
						oidStr := strings.TrimPrefix(v.Name, ".")
						name, _ := s.snmpRepo.TranslateOid(ctx, oidStr)
						valStr, typeStr := formatVarbind(v)
						results = append(results, domain.SnmpQueryResult{
							OID:   oidStr,
							Name:  name,
							Value: valStr,
							Type:  typeStr,
						})
					}
				}
				if len(results) > 0 {
					return results, nil
				}
			}

			// 2. Try walking the parent subtree (without .0)
			parentOid := strings.TrimSuffix(cleanOid, ".0")
			if snmpVersion == gosnmp.Version2c {
				_ = params.BulkWalk(parentOid, walkFn)
			} else {
				_ = params.Walk(parentOid, walkFn)
			}
		}

		if limitReached {
			return results, nil
		}

		if walkErr != nil && len(results) == 0 {
			return nil, s.enrichSnmpError(walkErr, host, port, community)
		}
	}

	return results, nil
}

func (s *SnmpService) enrichSnmpError(err error, host string, port uint16, community string) error {
	errStr := err.Error()
	if strings.Contains(strings.ToLower(errStr), "timeout") {
		return fmt.Errorf("SNMP query timed out connecting to %s:%d. Possible reasons: (1) Community string '%s' is rejected (SNMP agents silently ignore queries with invalid community strings), (2) UDP port %d is blocked or filtered by firewall/ACL, (3) The SNMP agent service is not running or not bound to this interface.", host, port, community, port)
	}
	return err
}

func (s *SnmpService) ImportMibText(ctx context.Context, mibName, content string) (*domain.ImportedMib, error) {
	// Write file to disk
	safeName := regexp.MustCompile(`[^a-zA-Z0-9_-]`).ReplaceAllString(mibName, "")
	if safeName == "" {
		safeName = "CUSTOM-MIB"
	}
	filePath := filepath.Join(s.mibsDir, safeName+".mib")
	_ = os.WriteFile(filePath, []byte(content), 0644)

	// Parse MIB syntax into OID definitions
	parsedNodes := parseMibSyntax(content, safeName)

	// 1. Foreign key constraint: imported_mibs MUST be saved before oid_registry references it!
	if err := s.snmpRepo.SaveImportedMib(ctx, safeName, len(parsedNodes)); err != nil {
		return nil, fmt.Errorf("failed to register MIB '%s': %w", safeName, err)
	}

	// 2. Save OID batch (deduplicated)
	if err := s.snmpRepo.SaveOidBatch(ctx, parsedNodes); err != nil {
		return nil, fmt.Errorf("failed to save OID definitions: %w", err)
	}

	logger.Info("SNMP", fmt.Sprintf("Imported MIB '%s' with %d OID definitions", safeName, len(parsedNodes)))

	return &domain.ImportedMib{
		Name:       safeName,
		NodeCount:  len(parsedNodes),
		ImportedAt: time.Now(),
	}, nil
}

func (s *SnmpService) seedBundledMibs() {
	bundledCandidates := []string{"/app/bundled_mibs", "./data/mibs"}
	var sourceDir string
	for _, dir := range bundledCandidates {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		absMibsDir, _ := filepath.Abs(s.mibsDir)
		if absDir == absMibsDir {
			continue
		}
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			sourceDir = dir
			break
		}
	}

	if sourceDir == "" {
		return
	}

	_ = filepath.WalkDir(sourceDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil {
			return nil
		}
		rel, err := filepath.Rel(sourceDir, path)
		if err != nil || rel == "." {
			return nil
		}

		destPath := filepath.Join(s.mibsDir, rel)
		if d.IsDir() {
			_ = os.MkdirAll(destPath, 0755)
			return nil
		}

		if _, statErr := os.Stat(destPath); os.IsNotExist(statErr) {
			if srcData, readErr := os.ReadFile(path); readErr == nil {
				_ = os.WriteFile(destPath, srcData, 0644)
			}
		}
		return nil
	})
}

func (s *SnmpService) SyncMibsFromDisk(ctx context.Context) {
	// 1. Ensure bundled MIBs are seeded if s.mibsDir is missing them
	s.seedBundledMibs()

	// 2. Fetch already imported MIBs from database to skip redundant parsing
	imported, err := s.snmpRepo.ListImportedMibs(ctx)
	if err != nil {
		logger.Warn("SNMP", fmt.Sprintf("Failed to list imported MIBs for sync: %v", err))
		return
	}

	existingSet := make(map[string]bool, len(imported))
	for _, m := range imported {
		existingSet[m.Name] = true
	}

	defRegex := regexp.MustCompile(`(?m)^\s*([a-zA-Z0-9_-]+)\s+DEFINITIONS\s*::=\s*BEGIN`)
	safeNameRegex := regexp.MustCompile(`[^a-zA-Z0-9_-]`)

	var filesToImport []string
	var namesToImport []string

	// 3. Walk s.mibsDir recursively to find all MIB definitions (vendor directories + root)
	_ = filepath.WalkDir(s.mibsDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d == nil {
			return nil
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}

		baseName := d.Name()
		if strings.HasPrefix(baseName, ".") || baseName == "README" || baseName == ".gitkeep" {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(baseName))
		if ext == ".md" || ext == ".json" || ext == ".yaml" || ext == ".yml" || ext == ".png" || ext == ".jpg" || ext == ".csv" || ext == ".svg" {
			return nil
		}

		cleanBase := strings.TrimSuffix(baseName, filepath.Ext(baseName))
		safeBase := safeNameRegex.ReplaceAllString(cleanBase, "")

		// Fast check by base name
		if safeBase != "" && existingSet[safeBase] {
			return nil
		}

		// Read first 2KB to check DEFINITIONS module name
		f, openErr := os.Open(path)
		if openErr != nil {
			return nil
		}
		buf := make([]byte, 2048)
		n, _ := f.Read(buf)
		_ = f.Close()

		modName := safeBase
		if m := defRegex.FindSubmatch(buf[:n]); len(m) > 1 {
			extracted := safeNameRegex.ReplaceAllString(string(m[1]), "")
			if extracted != "" {
				modName = extracted
			}
		}

		if modName == "" {
			return nil
		}

		if existingSet[modName] {
			return nil
		}

		// Deduplicate across vendor directories
		existingSet[modName] = true
		filesToImport = append(filesToImport, path)
		namesToImport = append(namesToImport, modName)
		return nil
	})

	if len(filesToImport) == 0 {
		logger.Info("SNMP", fmt.Sprintf("All bundled MIB modules are indexed and up to date (%d modules).", len(imported)))
		return
	}

	logger.Info("SNMP", fmt.Sprintf("Indexing %d new MIB modules from disk into database...", len(filesToImport)))

	importedCount := 0
	for i, path := range filesToImport {
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			continue
		}
		name := namesToImport[i]

		// Parse MIB syntax into OID definitions
		parsedNodes := parseMibSyntax(string(content), name)

		// 1. Foreign key constraint: imported_mibs MUST be saved before oid_registry
		if err := s.snmpRepo.SaveImportedMib(ctx, name, len(parsedNodes)); err != nil {
			logger.Warn("SNMP", fmt.Sprintf("Failed to register MIB '%s': %v", name, err))
			continue
		}

		// 2. Save OID batch
		if len(parsedNodes) > 0 {
			if err := s.snmpRepo.SaveOidBatch(ctx, parsedNodes); err != nil {
				logger.Warn("SNMP", fmt.Sprintf("Failed to save OID definitions for '%s': %v", name, err))
			}
		}

		importedCount++
		if importedCount%250 == 0 || importedCount == len(filesToImport) {
			logger.Info("SNMP", fmt.Sprintf("Indexed %d/%d MIB modules into database", importedCount, len(filesToImport)))
		}
	}

	logger.Info("SNMP", fmt.Sprintf("Finished syncing MIBs: %d new modules successfully registered.", importedCount))
}

type rawMibEntry struct {
	name        string
	parent      string
	index       string
	description *string
	syntax      *string
	access      *string
}

func parseMibSyntax(text, mibName string) []domain.OidRegistry {
	// Base symbol table initialized with well-known standard root OIDs
	symbols := map[string]string{
		"ccitt":        "0",
		"iso":          "1",
		"org":          "1.3",
		"dod":          "1.3.6",
		"internet":     "1.3.6.1",
		"directory":    "1.3.6.1.1",
		"mgmt":         "1.3.6.1.2",
		"mib-2":        "1.3.6.1.2.1",
		"system":       "1.3.6.1.2.1.1",
		"interfaces":   "1.3.6.1.2.1.2",
		"at":           "1.3.6.1.2.1.3",
		"ip":           "1.3.6.1.2.1.4",
		"icmp":         "1.3.6.1.2.1.5",
		"tcp":          "1.3.6.1.2.1.6",
		"udp":          "1.3.6.1.2.1.7",
		"egp":          "1.3.6.1.2.1.8",
		"transmission": "1.3.6.1.2.1.10",
		"snmp":         "1.3.6.1.2.1.11",
		"experimental": "1.3.6.1.3",
		"private":      "1.3.6.1.4",
		"enterprises":  "1.3.6.1.4.1",
		"security":     "1.3.6.1.5",
		"snmpV2":       "1.3.6.1.6",
		"snmpModules":  "1.3.6.1.6.3",
		"host":         "1.3.6.1.2.1.25",
	}

	// Regex to match ASN.1 object definitions:
	// Example: nodeName OBJECT-TYPE ... ::= { parent 1 }
	re := regexp.MustCompile(`(?s)\b([a-zA-Z0-9_-]+)\s+(OBJECT-TYPE|OBJECT\s+IDENTIFIER|MODULE-IDENTITY|NOTIFICATION-TYPE|TRAP-TYPE|OBJECT-GROUP|NOTIFICATION-GROUP)\s+(.*?)::=\s*\{\s*([^}]+)\s*\}`)
	matches := re.FindAllStringSubmatch(text, -1)

	var rawEntries []rawMibEntry
	descRe := regexp.MustCompile(`(?s)DESCRIPTION\s+"([^"]*)"`)
	syntaxRe := regexp.MustCompile(`(?i)SYNTAX\s+([a-zA-Z0-9_() -]+)`)
	accessRe := regexp.MustCompile(`(?i)(?:MAX-ACCESS|ACCESS)\s+([a-zA-Z0-9_-]+)`)

	for _, m := range matches {
		if len(m) < 5 {
			continue
		}
		name := strings.TrimSpace(m[1])
		body := m[3]
		refTokens := strings.Fields(strings.TrimSpace(m[4]))
		if len(refTokens) == 0 {
			continue
		}

		// Extract metadata
		var desc *string
		if dm := descRe.FindStringSubmatch(body); len(dm) > 1 {
			trimmedDesc := strings.TrimSpace(dm[1])
			if trimmedDesc != "" {
				desc = &trimmedDesc
			}
		}

		var syntax *string
		if sm := syntaxRe.FindStringSubmatch(body); len(sm) > 1 {
			s := strings.TrimSpace(sm[1])
			syntax = &s
		}

		var access *string
		if am := accessRe.FindStringSubmatch(body); len(am) > 1 {
			a := strings.TrimSpace(am[1])
			access = &a
		}

		if len(refTokens) >= 2 {
			parent := refTokens[0]
			idxToken := refTokens[1]
			idxStr := idxToken
			if pIdx := strings.Index(idxToken, "("); pIdx != -1 {
				idxStr = strings.Trim(idxToken[pIdx+1:], ")")
			}
			rawEntries = append(rawEntries, rawMibEntry{
				name:        name,
				parent:      parent,
				index:       idxStr,
				description: desc,
				syntax:      syntax,
				access:      access,
			})
		} else if len(refTokens) == 1 {
			rawEntries = append(rawEntries, rawMibEntry{
				name:        name,
				parent:      refTokens[0],
				index:       "0",
				description: desc,
				syntax:      syntax,
				access:      access,
			})
		}
	}

	// Multi-pass resolution of parent-child hierarchy
	unresolved := rawEntries
	for pass := 0; pass < 20; pass++ {
		var nextUnresolved []rawMibEntry
		resolvedCount := 0

		for _, item := range unresolved {
			if parentOid, ok := symbols[item.parent]; ok {
				var fullOid string
				if item.index == "0" && strings.HasSuffix(parentOid, ".0") {
					fullOid = parentOid
				} else {
					fullOid = parentOid + "." + item.index
				}
				symbols[item.name] = fullOid
				resolvedCount++
			} else {
				nextUnresolved = append(nextUnresolved, item)
			}
		}

		unresolved = nextUnresolved
		if resolvedCount == 0 || len(unresolved) == 0 {
			break
		}
	}

	// For any remaining unresolved nodes, assign deterministic unique fallbacks
	for i, item := range unresolved {
		symbols[item.name] = fmt.Sprintf("1.3.6.1.4.1.0.%d.%s", i+1, item.index)
	}

	// Build deduplicated results
	seenOids := make(map[string]domain.OidRegistry)
	for _, item := range rawEntries {
		oid, ok := symbols[item.name]
		if !ok || oid == "" {
			continue
		}
		cleanOid := strings.Trim(oid, ".")
		seenOids[cleanOid] = domain.OidRegistry{
			OID:         cleanOid,
			Name:        item.name,
			MibName:     mibName,
			Syntax:      item.syntax,
			Access:      item.access,
			Description: item.description,
		}
	}

	result := make([]domain.OidRegistry, 0, len(seenOids))
	for _, o := range seenOids {
		result = append(result, o)
	}

	return result
}

func formatVarbind(pdu gosnmp.SnmpPDU) (string, string) {
	switch pdu.Type {
	case gosnmp.OctetString:
		if bytes, ok := pdu.Value.([]byte); ok {
			return string(bytes), "OctetString"
		}
		return fmt.Sprintf("%v", pdu.Value), "OctetString"
	case gosnmp.Integer:
		return fmt.Sprintf("%v", pdu.Value), "Integer"
	case gosnmp.Counter32, gosnmp.Counter64:
		return fmt.Sprintf("%v", pdu.Value), "Counter"
	case gosnmp.Gauge32:
		return fmt.Sprintf("%v", pdu.Value), "Gauge"
	case gosnmp.TimeTicks:
		return fmt.Sprintf("%v", pdu.Value), "TimeTicks"
	case gosnmp.IPAddress:
		return fmt.Sprintf("%v", pdu.Value), "IPAddress"
	case gosnmp.ObjectIdentifier:
		return fmt.Sprintf("%v", pdu.Value), "OID"
	default:
		return fmt.Sprintf("%v", pdu.Value), fmt.Sprintf("%v", pdu.Type)
	}
}
