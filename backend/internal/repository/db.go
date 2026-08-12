
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


type Options struct {
	Path        string
	AutoMigrate bool
	LogQueries  bool
	Logger      *slog.Logger
}


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


func dsn(path string) string {
	pragmas := url.Values{}
	pragmas.Add("_pragma", "journal_mode(WAL)")
	pragmas.Add("_pragma", "busy_timeout(5000)")
	pragmas.Add("_pragma", "foreign_keys(1)")
	pragmas.Add("_pragma", "synchronous(NORMAL)")
	return "file:" + path + "?" + pragmas.Encode()
}


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
