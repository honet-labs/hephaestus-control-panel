package services

import "go-hephaestus/internal/core/domain"

func samePrometheusSSHIdentity(a, b domain.PrometheusConfig) bool {
	value := func(s *string) string {
		if s == nil {
			return ""
		}
		return *s
	}
	port := func(p *int) int {
		if p == nil || *p == 0 {
			return 22
		}
		return *p
	}
	return value(a.SSHHost) == value(b.SSHHost) && value(a.SSHUser) == value(b.SSHUser) && port(a.SSHPort) == port(b.SSHPort) && value(a.SSHAuth) == value(b.SSHAuth)
}
