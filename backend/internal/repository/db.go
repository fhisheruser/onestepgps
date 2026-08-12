// Package repository implements the domain persistence ports on top of GORM
// and SQLite. The rest of the application never sees a *gorm.DB.
package repository

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// Options configures the database connection.
type Options struct {
	Path        string
	AutoMigrate bool
	LogQueries  bool
	Logger      *slog.Logger
}

// Open connects to SQLite, applies the pragmas a concurrent web service needs
// and (optionally) runs migrations.
//
// The driver is glebarez/sqlite, a pure-Go port: no CGO, which means a static
// binary, a scratch-based Docker image and no C toolchain on any dev machine.
func Open(opts Options) (*gorm.DB, error) {
	if opts.Path == "" {
		return nil, fmt.Errorf("database path must not be empty")
	}
	if dir := filepath.Dir(opts.Path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create database directory %q: %w", dir, err)
		}
	}

	level := gormlogger.Silent
	if opts.LogQueries {
		level = gormlogger.Info
	}

	db, err := gorm.Open(sqlite.Open(dsn(opts.Path)), &gorm.Config{
		Logger:                 gormlogger.Default.LogMode(level),
		SkipDefaultTransaction: true,
		NowFunc:                func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		return nil, fmt.Errorf("open sqlite %q: %w", opts.Path, err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("access sql db: %w", err)
	}
	// SQLite serialises writes; a small pool with a single writer avoids
	// "database is locked" without throttling reads meaningfully at this scale.
	sqlDB.SetMaxOpenConns(4)
	sqlDB.SetMaxIdleConns(4)
	sqlDB.SetConnMaxLifetime(time.Hour)

	if opts.AutoMigrate {
		if err := Migrate(db); err != nil {
			return nil, err
		}
	}
	if opts.Logger != nil {
		opts.Logger.Info("database ready", "path", opts.Path, "migrated", opts.AutoMigrate)
	}
	return db, nil
}

// dsn builds the connection string, enabling WAL (concurrent readers during a
// write), a busy timeout (retry instead of failing instantly on lock
// contention) and foreign key enforcement, which SQLite disables by default.
func dsn(path string) string {
	pragmas := url.Values{}
	pragmas.Add("_pragma", "journal_mode(WAL)")
	pragmas.Add("_pragma", "busy_timeout(5000)")
	pragmas.Add("_pragma", "foreign_keys(1)")
	pragmas.Add("_pragma", "synchronous(NORMAL)")
	return "file:" + path + "?" + pragmas.Encode()
}

// Migrate creates or updates every table the application owns.
func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&userSettingsRecord{},
		&devicePreferenceRecord{},
		&iconRecord{},
		&historyPointRecord{},
	); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}

// Close releases the underlying connection pool.
func Close(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
