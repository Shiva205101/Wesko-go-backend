package dbase

import (
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
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
	db, err := gorm.Open(postgres.Open(cfg.ConnectString()), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	if cfg.Debug {
		db = db.Debug()
	}
	return db, nil
}
