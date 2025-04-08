package db

import "database/sql"

type DBProvider interface {
	Connect() (*sql.DB, error)
}
