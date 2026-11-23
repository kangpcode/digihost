// DigiPanel Versioned Database Migrator (core/dbstore/migrator.go)
// Executes embedded forward and backward SQL migrations inside PostgreSQL
// transactions and tracks applied versions in schema_migrations.
// Zero emojis, strictly production grade.

package dbstore

import (
	"context"
	"embed"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Migration represents one versioned up/down SQL step.
type Migration struct {
	Version int
	Name    string
	UpSQL   string
	DownSQL string
}

// AppliedMigration records one applied database migration row.
type AppliedMigration struct {
	Version int
	Name    string
}

func loadMigrations() ([]Migration, error) {
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return nil, fmt.Errorf("read embedded migrations: %w", err)
	}

	byVersion := make(map[int]*Migration)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		filename := entry.Name()
		isUp := strings.HasSuffix(filename, ".up.sql")
		isDown := strings.HasSuffix(filename, ".down.sql")
		if !isUp && !isDown {
			continue
		}

		parts := strings.SplitN(filename, "_", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid migration filename %q", filename)
		}
		version, err := strconv.Atoi(parts[0])
		if err != nil {
			return nil, fmt.Errorf("parse migration version %q: %w", filename, err)
		}

		name := strings.TrimSuffix(strings.TrimSuffix(parts[1], ".up.sql"), ".down.sql")
		raw, err := migrationFS.ReadFile("migrations/" + filename)
		if err != nil {
			return nil, fmt.Errorf("read migration %q: %w", filename, err)
		}

		item, exists := byVersion[version]
		if !exists {
			item = &Migration{Version: version, Name: name}
			byVersion[version] = item
		}
		if isUp {
			item.UpSQL = string(raw)
		} else {
			item.DownSQL = string(raw)
		}
	}

	out := make([]Migration, 0, len(byVersion))
	for _, item := range byVersion {
		if strings.TrimSpace(item.UpSQL) == "" || strings.TrimSpace(item.DownSQL) == "" {
			return nil, fmt.Errorf("migration %d (%s) must define both .up.sql and .down.sql", item.Version, item.Name)
		}
		out = append(out, *item)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Version < out[j].Version
	})
	return out, nil
}

func ensureMigrationsTable(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INT PRIMARY KEY,
			name VARCHAR(128) NOT NULL,
			applied_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	return err
}

// Applied returns applied migration versions ordered ascending.
func Applied(ctx context.Context, pool *pgxpool.Pool) ([]AppliedMigration, error) {
	if err := ensureMigrationsTable(ctx, pool); err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, `SELECT version, name FROM schema_migrations ORDER BY version ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AppliedMigration
	for rows.Next() {
		var m AppliedMigration
		if err := rows.Scan(&m.Version, &m.Name); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// MigrateUp applies all pending migrations in order.
func MigrateUp(ctx context.Context, pool *pgxpool.Pool) (int, error) {
	migrations, err := loadMigrations()
	if err != nil {
		return 0, err
	}
	if err := ensureMigrationsTable(ctx, pool); err != nil {
		return 0, err
	}

	appliedList, err := Applied(ctx, pool)
	if err != nil {
		return 0, err
	}
	applied := make(map[int]bool, len(appliedList))
	for _, item := range appliedList {
		applied[item.Version] = true
	}

	count := 0
	for _, m := range migrations {
		if applied[m.Version] {
			continue
		}
		if err := execMigrationTx(ctx, pool, m.UpSQL, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version, name) VALUES ($1, $2)`, m.Version, m.Name)
			return err
		}); err != nil {
			return count, fmt.Errorf("apply migration %04d_%s: %w", m.Version, m.Name, err)
		}
		count++
	}
	return count, nil
}

// MigrateDown rolls back migrations whose version is strictly greater than targetVersion.
// Pass targetVersion = 0 to roll back every applied migration.
func MigrateDown(ctx context.Context, pool *pgxpool.Pool, targetVersion int) (int, error) {
	migrations, err := loadMigrations()
	if err != nil {
		return 0, err
	}
	if err := ensureMigrationsTable(ctx, pool); err != nil {
		return 0, err
	}

	appliedList, err := Applied(ctx, pool)
	if err != nil {
		return 0, err
	}
	applied := make(map[int]bool, len(appliedList))
	for _, item := range appliedList {
		applied[item.Version] = true
	}

	count := 0
	for i := len(migrations) - 1; i >= 0; i-- {
		m := migrations[i]
		if m.Version <= targetVersion || !applied[m.Version] {
			continue
		}
		if err := execMigrationTx(ctx, pool, m.DownSQL, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `DELETE FROM schema_migrations WHERE version = $1`, m.Version)
			return err
		}); err != nil {
			return count, fmt.Errorf("rollback migration %04d_%s: %w", m.Version, m.Name, err)
		}
		count++
	}
	return count, nil
}

func execMigrationTx(ctx context.Context, pool *pgxpool.Pool, sqlText string, recordFn func(pgx.Tx) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, sqlText); err != nil {
		return err
	}
	if err := recordFn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
