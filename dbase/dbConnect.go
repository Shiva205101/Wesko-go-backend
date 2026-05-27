package dbase

import (
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type DBConfig struct {
	Host     string
	Password string
	Port     string
	User     string
	DBName   string
	SSLMode  string
	Debug    bool
}

func (cfg *DBConfig) ConnectString() string {
	var str string

	str = fmt.Sprintf(`host=%v port=%v user=%v dbname=%v password=%v sslmode=%v`,
		cfg.Host,
		cfg.Port,
		cfg.User,
		cfg.DBName,
		cfg.Password,
		cfg.SSLMode,
	)

	return str
}

func (cfg *DBConfig) DBConn() (*gorm.DB, error) {
	gormConfig := &gorm.Config{}
	if !cfg.Debug {
		gormConfig.Logger = logger.Default.LogMode(logger.Silent)
	}

	db, err := gorm.Open(postgres.Open(cfg.ConnectString()), gormConfig)
	if err != nil {
		return nil, err
	}
	return db, nil
}
