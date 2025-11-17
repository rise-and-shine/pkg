package outbox

import (
	"database/sql"
	"embed"
	"fmt"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// MigrationConfig configures migration behavior
type MigrationConfig struct {
	// TableName is the name of the outbox table (default: "outbox_events")
	TableName string

	// OffsetTableName is the name of the offset tracking table (default: "outbox_offset")
	OffsetTableName string

	// SkipIfExists skips migration if tables already exist (default: true)
	SkipIfExists bool
}

// DefaultMigrationConfig returns default configuration
func DefaultMigrationConfig() MigrationConfig {
	return MigrationConfig{
		TableName:       "outbox_events",
		OffsetTableName: "outbox_offset",
		SkipIfExists:    true,
	}
}

// RunMigrations executes the embedded SQL migrations
// This is a simple migration runner - for production, use a proper migration tool like golang-migrate or goose
func RunMigrations(db *sql.DB, cfg MigrationConfig) error {
	if cfg.TableName == "" {
		cfg = DefaultMigrationConfig()
	}

	// Check if table already exists
	if cfg.SkipIfExists {
		exists, err := tableExists(db, cfg.TableName)
		if err != nil {
			return fmt.Errorf("failed to check if table exists: %w", err)
		}
		if exists {
			return nil // Table already exists, skip migration
		}
	}

	// Read migration file
	migrationSQL, err := migrationsFS.ReadFile("migrations/001_enhanced_outbox.sql")
	if err != nil {
		return fmt.Errorf("failed to read migration file: %w", err)
	}

	// Execute migration
	_, err = db.Exec(string(migrationSQL))
	if err != nil {
		return fmt.Errorf("failed to execute migration: %w", err)
	}

	return nil
}

// GetMigrationSQL returns the SQL migration content for custom usage
func GetMigrationSQL() (string, error) {
	content, err := migrationsFS.ReadFile("migrations/001_enhanced_outbox.sql")
	if err != nil {
		return "", fmt.Errorf("failed to read migration file: %w", err)
	}
	return string(content), nil
}

// tableExists checks if a table exists in the database
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

// MigrationHelper provides utilities for working with migrations
type MigrationHelper struct {
	db  *sql.DB
	cfg MigrationConfig
}

// NewMigrationHelper creates a new migration helper
func NewMigrationHelper(db *sql.DB, cfg MigrationConfig) *MigrationHelper {
	if cfg.TableName == "" {
		cfg = DefaultMigrationConfig()
	}
	return &MigrationHelper{
		db:  db,
		cfg: cfg,
	}
}

// EnsureTables ensures outbox tables exist, creates them if they don't
func (m *MigrationHelper) EnsureTables() error {
	return RunMigrations(m.db, m.cfg)
}

// GetTableStatus returns information about the outbox tables
func (m *MigrationHelper) GetTableStatus() (map[string]bool, error) {
	status := make(map[string]bool)

	// Check main table
	exists, err := tableExists(m.db, m.cfg.TableName)
	if err != nil {
		return nil, err
	}
	status[m.cfg.TableName] = exists

	// Check offset table
	exists, err = tableExists(m.db, m.cfg.OffsetTableName)
	if err != nil {
		return nil, err
	}
	status[m.cfg.OffsetTableName] = exists

	return status, nil
}

// DropTables drops the outbox tables (use with caution!)
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

// GetTableRowCount returns the number of rows in the outbox table
func (m *MigrationHelper) GetTableRowCount() (int64, error) {
	var count int64
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", m.cfg.TableName)
	err := m.db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get row count: %w", err)
	}
	return count, nil
}

// GetPendingEventCount returns the number of pending events
func (m *MigrationHelper) GetPendingEventCount() (int64, error) {
	var count int64
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE status = 'pending'", m.cfg.TableName)
	err := m.db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get pending event count: %w", err)
	}
	return count, nil
}
