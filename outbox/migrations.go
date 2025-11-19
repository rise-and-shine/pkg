package outbox

import (
	"database/sql"
	"embed"
	"fmt"
)

var migrationsFS embed.FS

type MigrationConfig struct {
	TableName string

	OffsetTableName string

	SkipIfExists bool
}

func DefaultMigrationConfig() MigrationConfig {
	return MigrationConfig{
		TableName:       "outbox_events",
		OffsetTableName: "outbox_offset",
		SkipIfExists:    true,
	}
}

func RunMigrations(db *sql.DB, cfg MigrationConfig) error {
	if cfg.TableName == "" {
		cfg = DefaultMigrationConfig()
	}

	if cfg.SkipIfExists {
		exists, err := tableExists(db, cfg.TableName)
		if err != nil {
			return fmt.Errorf("failed to check if table exists: %w", err)
		}
		if exists {
			return nil
		}
	}

	migrationSQL, err := migrationsFS.ReadFile("migrations/001_enhanced_outbox.sql")
	if err != nil {
		return fmt.Errorf("failed to read migration file: %w", err)
	}

	_, err = db.Exec(string(migrationSQL))
	if err != nil {
		return fmt.Errorf("failed to execute migration: %w", err)
	}

	return nil
}

func GetMigrationSQL() (string, error) {
	content, err := migrationsFS.ReadFile("migrations/001_enhanced_outbox.sql")
	if err != nil {
		return "", fmt.Errorf("failed to read migration file: %w", err)
	}
	return string(content), nil
}

func tableExists(db *sql.DB, tableName string) (bool, error) {
	var exists bool
	query := `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = $1
		);
	`
	err := db.QueryRow(query, tableName).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check table existence: %w", err)
	}
	return exists, nil
}

type MigrationHelper struct {
	db  *sql.DB
	cfg MigrationConfig
}

func NewMigrationHelper(db *sql.DB, cfg MigrationConfig) *MigrationHelper {
	if cfg.TableName == "" {
		cfg = DefaultMigrationConfig()
	}
	return &MigrationHelper{
		db:  db,
		cfg: cfg,
	}
}

func (m *MigrationHelper) EnsureTables() error {
	return RunMigrations(m.db, m.cfg)
}

func (m *MigrationHelper) GetTableStatus() (map[string]bool, error) {
	status := make(map[string]bool)

	exists, err := tableExists(m.db, m.cfg.TableName)
	if err != nil {
		return nil, err
	}
	status[m.cfg.TableName] = exists

	exists, err = tableExists(m.db, m.cfg.OffsetTableName)
	if err != nil {
		return nil, err
	}
	status[m.cfg.OffsetTableName] = exists

	return status, nil
}

func (m *MigrationHelper) DropTables() error {
	queries := []string{
		fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE;", m.cfg.OffsetTableName),
		fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE;", m.cfg.TableName),
		"DROP FUNCTION IF EXISTS update_updated_at_column() CASCADE;",
	}

	for _, query := range queries {
		if _, err := m.db.Exec(query); err != nil {
			return fmt.Errorf("failed to drop table: %w", err)
		}
	}

	return nil
}

func (m *MigrationHelper) GetTableRowCount() (int64, error) {
	var count int64
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", m.cfg.TableName)
	err := m.db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get row count: %w", err)
	}
	return count, nil
}

func (m *MigrationHelper) GetPendingEventCount() (int64, error) {
	var count int64
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE status = 'pending'", m.cfg.TableName)
	err := m.db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get pending event count: %w", err)
	}
	return count, nil
}
