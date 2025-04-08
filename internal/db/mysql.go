package db

import (
	"database/sql"
	"fmt"
)

type MySQLConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
}

type MySQL struct {
	Config MySQLConfig
}

func (m *MySQL) Connect() (*sql.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
		m.Config.User, m.Config.Password,
		m.Config.Host, m.Config.Port,
		m.Config.DBName,
	)
	return sql.Open("mysql", dsn)
}
