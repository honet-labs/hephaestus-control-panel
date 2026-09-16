package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/logger"
	"go-hephaestus/internal/repository"

	"github.com/gosnmp/gosnmp"
)

type SnmpService struct {
	snmpRepo *repository.SnmpRepository
	mibsDir  string
}

func NewSnmpService(snmpRepo *repository.SnmpRepository, dataDir string) *SnmpService {
	mibsDir := filepath.Join(dataDir, "mibs")
	_ = os.MkdirAll(mibsDir, 0755)

	s := &SnmpService{
		snmpRepo: snmpRepo,
		mibsDir:  mibsDir,
	}

	go s.SyncMibsFromDisk(context.Background())
	return s
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
	cleanOid := strings.Trim(startOid, ".")
	if cleanOid == "" {
		cleanOid = "1.3.6.1.2.1.1" // default MIB-2 system group
	}
	if !strings.HasPrefix(cleanOid, ".") {
		cleanOid = "." + cleanOid
	}

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

		walkFn := func(dataUnit gosnmp.SnmpPDU) error {
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
		} else {
			walkErr = params.Walk(cleanOid, walkFn)
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
	filePath := filepath.Join(s.mibsDir, safeName+".mib")
	_ = os.WriteFile(filePath, []byte(content), 0644)

	// Parse MIB syntax
	parsedNodes := parseMibSyntax(content, safeName)
	if err := s.snmpRepo.SaveOidBatch(ctx, parsedNodes); err != nil {
		return nil, err
	}

	_ = s.snmpRepo.SaveImportedMib(ctx, safeName, len(parsedNodes))
	logger.Info("SNMP", fmt.Sprintf("Imported MIB '%s' with %d OID definitions", safeName, len(parsedNodes)))

	return &domain.ImportedMib{
		Name:       safeName,
		NodeCount:  len(parsedNodes),
		ImportedAt: time.Now(),
	}, nil
}

func (s *SnmpService) SyncMibsFromDisk(ctx context.Context) {
	files, err := os.ReadDir(s.mibsDir)
	if err != nil {
		return
	}

	imported, _ := s.snmpRepo.ListImportedMibs(ctx)
	existingSet := make(map[string]bool)
	for _, m := range imported {
		existingSet[m.Name] = true
	}

	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".mib") {
			name := strings.TrimSuffix(f.Name(), ".mib")
			if !existingSet[name] {
				content, err := os.ReadFile(filepath.Join(s.mibsDir, f.Name()))
				if err == nil {
					_, _ = s.ImportMibText(ctx, name, string(content))
				}
			}
		}
	}
}

func parseMibSyntax(text, mibName string) []domain.OidRegistry {
	var oids []domain.OidRegistry
	// Basic regex extractor for MIB OBJECT-TYPE statements
	re := regexp.MustCompile(`(\w+)\s+(OBJECT-TYPE|OBJECT\s+IDENTIFIER|MODULE-IDENTITY)\s+(.*?)::=\s*\{\s*([\w-]+)\s+(\d+|\w+\(\d+\))\s*\}`)
	matches := re.FindAllStringSubmatch(text, -1)

	for _, m := range matches {
		if len(m) >= 6 {
			name := m[1]
			idxStr := m[5]
			idx, _ := strconv.Atoi(idxStr)
			descRe := regexp.MustCompile(`DESCRIPTION\s+"([^"]+)"`)
			descMatch := descRe.FindStringSubmatch(m[3])
			var desc *string
			if len(descMatch) > 1 {
				desc = &descMatch[1]
			}

			oidStr := fmt.Sprintf("1.3.6.1.4.1.%d", idx) // Simplified OID anchor
			oids = append(oids, domain.OidRegistry{
				OID:         oidStr,
				Name:        name,
				MibName:     mibName,
				Description: desc,
			})
		}
	}
	return oids
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
