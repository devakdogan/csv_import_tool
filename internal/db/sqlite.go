package db

import "database/sql"

type SQLiteConfig struct {
	FilePath string
}

type SQLite struct {
	Config SQLiteConfig
}

func (s *SQLite) Connect() (*sql.DB, error) {
	return sql.Open("sqlite", s.Config.FilePath)
}
