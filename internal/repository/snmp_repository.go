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

	batchSize := 100
	for i := 0; i < len(oids); i += batchSize {
		end := i + batchSize
		if end > len(oids) {
			end = len(oids)
		}
		chunk := oids[i:end]

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

	// Standard fallback prefix translations
	standardPrefixes := map[string]string{
		"1.3.6.1.2.1.1":  "system",
		"1.3.6.1.2.1.2":  "interfaces",
		"1.3.6.1.2.1.4":  "ip",
		"1.3.6.1.2.1.5":  "icmp",
		"1.3.6.1.2.1.6":  "tcp",
		"1.3.6.1.2.1.7":  "udp",
		"1.3.6.1.2.1.25": "hostResources",
		"1.3.6.1.4.1":    "enterprises",
	}

	for prefix, name := range standardPrefixes {
		if strings.HasPrefix(cleanOid, prefix) {
			suffix := strings.TrimPrefix(cleanOid, prefix)
			return name + suffix, nil
		}
	}

	return cleanOid, nil
}
