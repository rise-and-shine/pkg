package alert

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/code19m/errx"
	"github.com/google/uuid"
	"github.com/rise-and-shine/pkg/meta"
	"github.com/rise-and-shine/pkg/observability/logger"
	"github.com/uptrace/bun"
)

// errorRecord represents an error stored in the database.
type errorRecord struct {
	ID        string            `bun:"id,pk"`
	Code      string            `bun:"code,notnull"`
	Message   string            `bun:"message,notnull"`
	Details   map[string]string `bun:"details,type:jsonb,notnull"`
	Service   string            `bun:"service,notnull"`
	Operation string            `bun:"operation,notnull"`
	CreatedAt time.Time         `bun:"created_at,notnull"`
	Alerted   bool              `bun:"alerted,notnull"`
}

// errorInfo holds error data used for building notification messages.
type errorInfo struct {
	code      string
	message   string
	service   string
	operation string
	details   map[string]string

	frequency        int
	frequencyMinutes int
}

// --- Alert provider ---

// alertProvider implements the Provider interface by storing errors in PostgreSQL
// and sending notifications to Discord or Telegram with cooldown management.
type alertProvider struct {
	cfg      Config
	store    *store
	notifier notifier
}

func (ap *alertProvider) SendError(
	ctx context.Context,
	errCode, msg, operation string,
	details map[string]string,
) error {
	if details == nil {
		details = make(map[string]string)
	}
	details["service_version"] = meta.ServiceVersion()

	rec := errorRecord{
		ID:        uuid.NewString(),
		Code:      errCode,
		Message:   msg,
		Details:   details,
		Service:   meta.ServiceName(),
		Operation: operation,
		CreatedAt: time.Now(),
		Alerted:   false,
	}

	ctx = context.WithoutCancel(ctx)

	if err := ap.store.add(ctx, rec); err != nil {
		return err
	}

	go ap.processAlert(rec)

	return nil
}

func (ap *alertProvider) processAlert(rec errorRecord) {
	log := logger.Named("alert")
	ctx := context.Background()

	err := ap.store.checkAndMarkAlerted(ctx, rec, ap.cfg.CooldownMinutes)
	if err != nil {
		if errors.Is(err, errAlertCooldown) {
			return
		}
		log.With("error", err.Error()).Warn("checkAndMarkAlerted failed")
		return
	}

	frequency, err := ap.store.getErrorFrequency(ctx, rec.Service, rec.Operation, ap.cfg.CooldownMinutes)
	if err != nil {
		log.With("error", err.Error()).Warn("getErrorFrequency failed")
		return
	}

	info := errorInfo{
		code:             rec.Code,
		message:          rec.Message,
		service:          rec.Service,
		operation:        rec.Operation,
		details:          rec.Details,
		frequency:        frequency,
		frequencyMinutes: ap.cfg.CooldownMinutes,
	}

	notifyErr := ap.notifier.notify(ctx, info)
	if notifyErr != nil {
		log.With("error", notifyErr.Error()).Warn("notification failed")
	}
}

// --- PostgreSQL store ---

var errAlertCooldown = errors.New("alert is in cooldown period")

type store struct {
	db     *bun.DB
	table  string
	schema string
}

func newStore(db *bun.DB, schema string) (*store, error) {
	s := &store{
		db:     db,
		table:  fmt.Sprintf("%q.errors", schema),
		schema: schema,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.initDB(ctx); err != nil {
		return nil, errx.Wrap(err)
	}

	return s, nil
}

func (s *store) initDB(ctx context.Context) error {
	_, err := s.db.NewRaw(`
		CREATE SCHEMA IF NOT EXISTS ?;

		CREATE TABLE IF NOT EXISTS ? (
			id UUID PRIMARY KEY,
			code TEXT NOT NULL,
			message TEXT NOT NULL,
			details JSONB NOT NULL,
			service TEXT NOT NULL,
			operation TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL,
			alerted BOOLEAN NOT NULL
		);

		CREATE INDEX IF NOT EXISTS idx_errors_service_operation_alerted
		ON ? (service, operation, alerted);

		CREATE INDEX IF NOT EXISTS idx_errors_created_at
		ON ? (created_at);
	`, bun.Ident(s.schema), bun.Safe(s.table), bun.Safe(s.table), bun.Safe(s.table)).Exec(ctx)
	if err != nil {
		return errx.Wrap(err)
	}

	return nil
}

func (s *store) add(ctx context.Context, e errorRecord) error {
	_, err := s.db.NewRaw(`
		INSERT INTO ? (id, code, message, details, service, operation, created_at, alerted)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?);
	`, bun.Safe(s.table), e.ID, e.Code, e.Message, e.Details, e.Service, e.Operation, e.CreatedAt, e.Alerted).Exec(ctx)
	if err != nil {
		return errx.Wrap(err)
	}

	return nil
}

func (s *store) checkAndMarkAlerted(ctx context.Context, e errorRecord, cooldownMinutes int) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return errx.Wrap(err)
	}
	defer tx.Rollback() //nolint:errcheck // intentional

	// Acquire advisory lock based on service+operation hash.
	lockKey := hashServiceOperation(e.Service, e.Operation)
	_, err = tx.NewRaw(`SELECT pg_advisory_xact_lock(?)`, lockKey).Exec(ctx)
	if err != nil {
		return errx.Wrap(err)
	}

	// Check cooldown: find the most recent alerted error for this service+operation.
	var lastAlertedAt time.Time
	err = tx.NewRaw(`
		SELECT created_at
		FROM ?
		WHERE service = ? AND operation = ? AND alerted = true
		ORDER BY created_at DESC
		LIMIT 1
	`, bun.Safe(s.table), e.Service, e.Operation).Scan(ctx, &lastAlertedAt)

	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return errx.Wrap(err)
	}

	if err == nil {
		cooldownDuration := time.Duration(cooldownMinutes) * time.Minute
		if time.Since(lastAlertedAt) < cooldownDuration {
			return errAlertCooldown
		}
	}

	// Cooldown passed, mark as alerted.
	_, err = tx.NewRaw(`
		UPDATE ?
		SET alerted = true
		WHERE id = ?
	`, bun.Safe(s.table), e.ID).Exec(ctx)
	if err != nil {
		return errx.Wrap(err)
	}

	commitErr := tx.Commit()
	if commitErr != nil {
		return errx.Wrap(commitErr)
	}

	return nil
}

func (s *store) getErrorFrequency(ctx context.Context, service, operation string, minutesBack int) (int, error) {
	var count int

	err := s.db.NewRaw(`
		SELECT COUNT(*)
		FROM ?
		WHERE service = ? AND operation = ? AND created_at > NOW() - INTERVAL '1 minute' * ?
	`, bun.Safe(s.table), service, operation, minutesBack).Scan(ctx, &count)
	if err != nil {
		return 0, errx.Wrap(err)
	}

	return count, nil
}

// hashServiceOperation creates a consistent hash for service+operation to use as advisory lock key.
func hashServiceOperation(service, operation string) int64 {
	combined := service + ":" + operation
	var hash int64
	for i, c := range combined {
		hash = hash*31 + int64(c) + int64(i)
	}
	if hash < 0 {
		hash = -hash
	}
	return hash
}
