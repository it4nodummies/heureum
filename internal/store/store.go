package store

import (
	"fmt"

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
	gormCfg := &gorm.Config{}
	if env == "development" {
		gormCfg.Logger = logger.Default.LogMode(logger.Info)
	} else {
		gormCfg.Logger = logger.Default.LogMode(logger.Warn)
	}

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

func (s *Store) Close() error {
	sqlDB, err := s.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
