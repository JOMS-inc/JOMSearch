package db 

import {
	"database/sql"

	_"modernc.org/sqlite"
}

func Open(path string) (*sql.DB, error) {
	conn, err :* sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	if err := conn.Ping(); err != nil {
		return nil, err
	}
	
	return conn, nil
}