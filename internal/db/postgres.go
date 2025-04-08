package db

import (
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
)

type PostgresConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

type Postgres struct {
	Config PostgresConfig
}

func (p *Postgres) Connect() (*sql.DB, error) {
	connStr := fmt.Sprintf("user=%s password=%s dbname=%s host=%s port=%d sslmode=%s",
		p.Config.User, p.Config.Password, p.Config.DBName, p.Config.Host, p.Config.Port, p.Config.SSLMode)
	return sql.Open("postgres", connStr)
}
