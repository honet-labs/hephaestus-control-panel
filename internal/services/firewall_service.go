package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/database"
	"go-hephaestus/internal/repository"

	"github.com/google/uuid"
)

type FirewallService struct {
	remoteRepo *repository.RemoteHostRepository
	sshService *SSHService
}

func NewFirewallService(remoteRepo *repository.RemoteHostRepository, sshService *SSHService) *FirewallService {
	return &FirewallService{
		remoteRepo: remoteRepo,
		sshService: sshService,
	}
}

func (s *FirewallService) executeWithSudo(cfg *domain.RemoteHostConfig, innerCmd string) (string, string, int, error) {
	var escapedPass string
	if cfg.Password != nil && *cfg.Password != "" {
		escapedPass = strings.ReplaceAll(*cfg.Password, "'", "'\\''")
	}
	cmd := fmt.Sprintf(`_P='%s'; _s() { if [ -n "$_P" ]; then echo "$_P" | sudo -S -p '' "$@" 2>/dev/null || sudo -n "$@" 2>/dev/null || "$@" 2>/dev/null; else sudo -n "$@" 2>/dev/null || "$@" 2>/dev/null; fi; }; %s`, escapedPass, innerCmd)
	return s.sshService.ExecuteCommand(cfg, cmd)
}

func (s *FirewallService) GetFirewallStatus(ctx context.Context, hostID string) (map[string]interface{}, error) {
	cfg, err := s.remoteRepo.GetRawByID(ctx, hostID)
	if err != nil {
		return nil, err
	}

	cmd := `(_s ufw status verbose 2>/dev/null || ufw status verbose 2>/dev/null); echo "===FIREWALLD==="; (_s firewall-cmd --state 2>/dev/null || firewall-cmd --state 2>/dev/null); echo "===IPTABLES==="; (_s iptables -S INPUT 2>/dev/null || iptables -S INPUT 2>/dev/null | head -n 30)`

	stdout, _, _, _ := s.executeWithSudo(cfg, cmd)

	active := false
	serviceName := "none"
	rawStatus := "Unknown status"

	parts := strings.Split(stdout, "===FIREWALLD===")
	ufwPart := strings.TrimSpace(parts[0])
	var fwdPart, iptPart string
	if len(parts) > 1 {
		p2 := strings.Split(parts[1], "===IPTABLES===")
		fwdPart = strings.TrimSpace(p2[0])
		if len(p2) > 1 {
			iptPart = strings.TrimSpace(p2[1])
		}
	}

	if strings.Contains(strings.ToLower(ufwPart), "status: active") {
		active = true
		serviceName = "ufw"
		rawStatus = "Active (UFW is active and enabled)"
	} else if strings.Contains(strings.ToLower(ufwPart), "status: inactive") {
		active = false
		serviceName = "ufw"
		rawStatus = "Inactive (UFW is disabled)"
	} else if strings.Contains(strings.ToLower(fwdPart), "running") {
		active = true
		serviceName = "firewalld"
		rawStatus = "Active (Firewalld is running)"
	} else if strings.Contains(strings.ToLower(fwdPart), "not running") {
		active = false
		serviceName = "firewalld"
		rawStatus = "Inactive (Firewalld is stopped)"
	} else if strings.Contains(iptPart, "-P INPUT") {
		serviceName = "iptables"
		active = true
		rawStatus = "Active (Iptables active packet filter)"
	} else {
		rawStatus = "No active firewall detected or root privileges required"
	}

	// Fetch rules from database
	rules, err := s.ListRules(ctx, hostID)
	if err != nil {
		rules = []domain.RemoteHostFirewallRule{}
	}

	// If no rules in DB and UFW output has rules, auto-import them
	if len(rules) == 0 && strings.Contains(ufwPart, "To") && strings.Contains(ufwPart, "Action") {
		parsedRules := parseUfwRules(ufwPart, hostID)
		for _, r := range parsedRules {
			_ = s.SaveRuleToDB(ctx, r)
		}
		if len(parsedRules) > 0 {
			rules = parsedRules
		}
	}

	return map[string]interface{}{
		"active":    active,
		"service":   serviceName,
		"rawStatus": rawStatus,
		"rules":     rules,
	}, nil
}

func parseUfwRules(output string, hostID string) []domain.RemoteHostFirewallRule {
	var rules []domain.RemoteHostFirewallRule
	lines := strings.Split(output, "\n")
	start := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "--") {
			start = true
			continue
		}
		if !start || line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		// e.g. "22/tcp ALLOW IN Anywhere" or "80 ALLOW IN 192.168.1.0/24"
		portProto := fields[0]
		action := "ALLOW"
		if strings.Contains(strings.ToUpper(line), "DENY") || strings.Contains(strings.ToUpper(line), "REJECT") {
			action = "DENY"
		}

		proto := "ALL"
		port := portProto
		if strings.Contains(portProto, "/") {
			pp := strings.Split(portProto, "/")
			port = pp[0]
			proto = strings.ToUpper(pp[1])
		}

		source := "0.0.0.0/0"
		for i := 1; i < len(fields); i++ {
			fUpper := strings.ToUpper(fields[i])
			if fUpper == "ANYWHERE" || fUpper == "ANYWHERE (V6)" {
				source = "0.0.0.0/0"
				break
			}
			if strings.Contains(fields[i], ".") || strings.Contains(fields[i], ":") {
				source = fields[i]
			}
		}

		rule := domain.RemoteHostFirewallRule{
			ID:          fmt.Sprintf("fwr-%s", uuid.New().String()[:8]),
			HostID:      hostID,
			Protocol:    proto,
			PortRange:   port,
			SourceIP:    source,
			Action:      action,
			Description: fmt.Sprintf("Imported from %s", portProto),
			IsActive:    true,
			CreatedAt:   time.Now(),
		}
		rules = append(rules, rule)
	}

	return rules
}

func (s *FirewallService) ListRules(ctx context.Context, hostID string) ([]domain.RemoteHostFirewallRule, error) {
	pool, err := database.GetPool()
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, host_id, protocol, port_range, source_ip, action, description, is_active, created_at
		FROM remote_host_firewall_rules
		WHERE host_id = $1
		ORDER BY created_at DESC
	`
	rows, err := pool.Query(ctx, query, hostID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []domain.RemoteHostFirewallRule
	for rows.Next() {
		var r domain.RemoteHostFirewallRule
		if err := rows.Scan(&r.ID, &r.HostID, &r.Protocol, &r.PortRange, &r.SourceIP, &r.Action, &r.Description, &r.IsActive, &r.CreatedAt); err != nil {
			continue
		}
		rules = append(rules, r)
	}

	return rules, nil
}

func (s *FirewallService) SaveRuleToDB(ctx context.Context, rule domain.RemoteHostFirewallRule) error {
	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	query := `
		INSERT INTO remote_host_firewall_rules (id, host_id, protocol, port_range, source_ip, action, description, is_active, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO UPDATE SET
			protocol = EXCLUDED.protocol,
			port_range = EXCLUDED.port_range,
			source_ip = EXCLUDED.source_ip,
			action = EXCLUDED.action,
			description = EXCLUDED.description,
			is_active = EXCLUDED.is_active
	`
	_, err = pool.Exec(ctx, query, rule.ID, rule.HostID, rule.Protocol, rule.PortRange, rule.SourceIP, rule.Action, rule.Description, rule.IsActive, rule.CreatedAt)
	return err
}

func (s *FirewallService) AddFirewallRule(ctx context.Context, hostID string, rule domain.RemoteHostFirewallRule) (*domain.RemoteHostFirewallRule, error) {
	cfg, err := s.remoteRepo.GetRawByID(ctx, hostID)
	if err != nil {
		return nil, err
	}

	if rule.ID == "" {
		rule.ID = fmt.Sprintf("fwr-%s", uuid.New().String()[:8])
	}
	rule.HostID = hostID
	rule.CreatedAt = time.Now()
	rule.IsActive = true

	if rule.Protocol == "" {
		rule.Protocol = "TCP"
	}
	rule.Protocol = strings.ToUpper(rule.Protocol)
	if rule.Action == "" {
		rule.Action = "ALLOW"
	}
	rule.Action = strings.ToUpper(rule.Action)
	if rule.SourceIP == "" {
		rule.SourceIP = "0.0.0.0/0"
	}
	if rule.PortRange == "" {
		rule.PortRange = "ALL"
	}

	// 1. Save to database
	if err := s.SaveRuleToDB(ctx, rule); err != nil {
		return nil, fmt.Errorf("failed to save rule in database: %w", err)
	}

	// 2. Apply on remote server via SSH
	actionLower := strings.ToLower(rule.Action) // allow or deny

	var cmd string
	port := strings.TrimSpace(rule.PortRange)
	protoLower := strings.ToLower(rule.Protocol)

	if rule.Protocol == "ICMP" || port == "ALL" {
		if rule.SourceIP == "0.0.0.0/0" || rule.SourceIP == "ALL" {
			cmd = fmt.Sprintf(`_s ufw %[1]s proto icmp 2>/dev/null || _s iptables -A INPUT -p icmp -j %[2]s 2>/dev/null`, actionLower, rule.Action)
		} else {
			cmd = fmt.Sprintf(`_s ufw %[1]s from %[2]s 2>/dev/null || _s iptables -A INPUT -s %[2]s -j %[3]s 2>/dev/null`, actionLower, rule.SourceIP, rule.Action)
		}
	} else {
		// Port based rule
		if rule.SourceIP == "0.0.0.0/0" || rule.SourceIP == "ALL" {
			if protoLower == "all" {
				cmd = fmt.Sprintf(`_s ufw %[1]s %[2]s 2>/dev/null || _s firewall-cmd --add-port=%[2]s/tcp --permanent 2>/dev/null`, actionLower, port)
			} else {
				cmd = fmt.Sprintf(`_s ufw %[1]s %[2]s/%[3]s 2>/dev/null || _s firewall-cmd --add-port=%[2]s/%[3]s --permanent 2>/dev/null || _s iptables -A INPUT -p %[3]s --dport %[2]s -j %[4]s 2>/dev/null`, actionLower, port, protoLower, rule.Action)
			}
		} else {
			if protoLower == "all" {
				cmd = fmt.Sprintf(`_s ufw %[1]s from %[2]s to any port %[3]s 2>/dev/null || _s iptables -A INPUT -s %[2]s -p tcp --dport %[3]s -j %[4]s 2>/dev/null`, actionLower, rule.SourceIP, port, rule.Action)
			} else {
				cmd = fmt.Sprintf(`_s ufw %[1]s proto %[2]s from %[3]s to any port %[4]s 2>/dev/null || _s iptables -A INPUT -s %[3]s -p %[2]s --dport %[4]s -j %[5]s 2>/dev/null`, actionLower, protoLower, rule.SourceIP, port, rule.Action)
			}
		}
	}

	_, _, _, _ = s.executeWithSudo(cfg, cmd)
	return &rule, nil
}

func (s *FirewallService) DeleteFirewallRule(ctx context.Context, hostID string, ruleID string) error {
	cfg, err := s.remoteRepo.GetRawByID(ctx, hostID)
	if err != nil {
		return err
	}

	pool, err := database.GetPool()
	if err != nil {
		return err
	}

	var r domain.RemoteHostFirewallRule
	query := `SELECT id, host_id, protocol, port_range, source_ip, action, description FROM remote_host_firewall_rules WHERE id = $1 AND host_id = $2`
	err = pool.QueryRow(ctx, query, ruleID, hostID).Scan(&r.ID, &r.HostID, &r.Protocol, &r.PortRange, &r.SourceIP, &r.Action, &r.Description)
	if err == nil {
		// Attempt removal via SSH
		actionLower := strings.ToLower(r.Action)
		protoLower := strings.ToLower(r.Protocol)
		port := strings.TrimSpace(r.PortRange)

		var cmd string
		if r.SourceIP == "0.0.0.0/0" || r.SourceIP == "ALL" {
			if protoLower == "all" {
				cmd = fmt.Sprintf(`_s ufw delete %[1]s %[2]s 2>/dev/null || _s firewall-cmd --remove-port=%[2]s/tcp --permanent 2>/dev/null`, actionLower, port)
			} else {
				cmd = fmt.Sprintf(`_s ufw delete %[1]s %[2]s/%[3]s 2>/dev/null || _s firewall-cmd --remove-port=%[2]s/%[3]s --permanent 2>/dev/null || _s iptables -D INPUT -p %[3]s --dport %[2]s -j %[4]s 2>/dev/null`, actionLower, port, protoLower, r.Action)
			}
		} else {
			cmd = fmt.Sprintf(`_s ufw delete %[1]s proto %[2]s from %[3]s to any port %[4]s 2>/dev/null || _s iptables -D INPUT -s %[3]s -p %[2]s --dport %[4]s -j %[5]s 2>/dev/null`, actionLower, protoLower, r.SourceIP, port, r.Action)
		}
		_, _, _, _ = s.executeWithSudo(cfg, cmd)
	}

	// Delete from DB
	delQuery := `DELETE FROM remote_host_firewall_rules WHERE id = $1 AND host_id = $2`
	_, err = pool.Exec(ctx, delQuery, ruleID, hostID)
	return err
}

func (s *FirewallService) ToggleFirewall(ctx context.Context, hostID string, enable bool) (string, error) {
	cfg, err := s.remoteRepo.GetRawByID(ctx, hostID)
	if err != nil {
		return "", err
	}

	var cmd string
	if enable {
		cmd = `_s ufw --force enable 2>/dev/null || _s systemctl start firewalld 2>/dev/null`
	} else {
		cmd = `_s ufw disable 2>/dev/null || _s systemctl stop firewalld 2>/dev/null`
	}

	stdout, stderr, _, err := s.executeWithSudo(cfg, cmd)
	if err != nil && stdout == "" {
		return stderr, err
	}
	return stdout + stderr, nil
}
