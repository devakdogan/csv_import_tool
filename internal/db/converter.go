package db

import (
	"fyne.io/fyne/v2/widget"
	"strconv"
)

type DbConfig struct {
	Host       *widget.Entry
	Port       *widget.Entry
	User       *widget.Entry
	Password   *widget.Entry
	Database   *widget.Entry
	Configured bool
}

// Dönüştürücüler
func (c *DbConfig) ToPostgresConfig() PostgresConfig {
	port, _ := strconv.Atoi(c.Port.Text)
	return PostgresConfig{
		Host:     c.Host.Text,
		Port:     port,
		User:     c.User.Text,
		Password: c.Password.Text,
		DBName:   c.Database.Text,
		SSLMode:  "disable",
	}
}

func (c *DbConfig) ToMySQLConfig() MySQLConfig {
	port, _ := strconv.Atoi(c.Port.Text)
	return MySQLConfig{
		Host:     c.Host.Text,
		Port:     port,
		User:     c.User.Text,
		Password: c.Password.Text,
		DBName:   c.Database.Text,
	}
}

func (c *DbConfig) ToSQLiteConfig() SQLiteConfig {
	return SQLiteConfig{
		FilePath: c.Database.Text,
	}
}
