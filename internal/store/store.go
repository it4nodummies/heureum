package store

import (
	"fmt"
	"log"
	"os"
	"time"

	gomysql "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/it4nodummies/heureum/internal/config"
)

// ensureMultiStatements returns dsn with multiStatements=true set. golang-migrate's
// MySQL driver executes each migration file as ONE statement, and our migration files
// contain many, so a connection without it fails with Error 1064 on a fresh database
// (see the comment above mysql.WithInstance in golang-migrate).
func ensureMultiStatements(dsn string) (string, error) {
	cfg, err := gomysql.ParseDSN(dsn)
	if err != nil {
		return "", err
	}
	cfg.MultiStatements = true
	return cfg.FormatDSN(), nil
}

type Store struct {
	DB     *gorm.DB
	Driver string
}

func New(cfg config.DBConfig, env string) (*Store, error) {
	gormCfg := &gorm.Config{Logger: newLogger(env)}

	var dialector gorm.Dialector
	switch cfg.Driver {
	case "postgres":
		dialector = postgres.Open(cfg.DSN)
	case "mysql", "mariadb":
		dsn, err := ensureMultiStatements(cfg.DSN)
		if err != nil {
			return nil, fmt.Errorf("failed to parse MySQL DSN: %w", err)
		}
		dialector = mysql.Open(dsn)
	case "sqlite":
		dialector = sqlite.Open(cfg.DSN)
	default:
		return nil, fmt.Errorf("unsupported DB driver: %s", cfg.Driver)
	}

	db, err := gorm.Open(dialector, gormCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)

	return &Store{DB: db, Driver: cfg.Driver}, nil
}

// newLogger builds the GORM logger. Outside development it logs at Warn
// (failed and slow queries only) with ParameterizedQueries: GORM would
// otherwise print the SQL with its bound values, and the statements that
// fail or are slow are exactly the ones carrying secrets — a webhook_secret on
// a failed provider INSERT, a bcrypt hash on a duplicate registration, a URL
// token on a lookup. ErrRecordNotFound is an expected outcome for First()
// lookups, not an error worth a log line. Development keeps the full Info
// trace with values, for local debugging only.
func newLogger(env string) logger.Interface {
	cfg := logger.Config{
		SlowThreshold:             200 * time.Millisecond,
		LogLevel:                  logger.Warn,
		IgnoreRecordNotFoundError: true,
		ParameterizedQueries:      true,
	}
	if env == "development" {
		cfg.LogLevel = logger.Info
		cfg.IgnoreRecordNotFoundError = false
		cfg.ParameterizedQueries = false
		cfg.Colorful = true
	}
	return logger.New(log.New(os.Stdout, "\r\n", log.LstdFlags), cfg)
}

func (s *Store) Close() error {
	sqlDB, err := s.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
