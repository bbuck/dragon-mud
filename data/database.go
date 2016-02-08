package data

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/jinzhu/gorm"

	// load database drivers
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

type databaseConfig struct {
	Adapter  string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

func (d *databaseConfig) valid() bool {
	if len(d.Adapter) == 0 {
		return false
	}

	if len(d.DBName) == 0 {
		return false
	}

	return true
}

func (d *databaseConfig) createDatabase() error {
	var connStr = d.connectionString(false)
	switch d.Adapter {
	case "sqlite3":
		os.Mkdir("data", os.ModePerm)
		if _, err := os.Open(connStr); err != nil {
			if os.IsNotExist(err) {
				os.Create(connStr)
			} else {
				return err
			}
		}

		return nil
	case "mysql":
		db, err := sql.Open(d.Adapter, connStr)
		if err != nil {
			return err
		}
		query := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", d.DBName)
		if _, err := db.Exec(query); err != nil {
			return err
		}
	case "postgres":
		db, err := sql.Open(d.Adapter, connStr)
		if err != nil {
			return err
		}
		query := fmt.Sprintf("CREATE DATABASE %s", d.DBName)
		if _, err := db.Exec(query); err != nil {
			// we ignore the error if the database already exists
			matches, _ := regexp.MatchString(`database\s+(.+)\s+already\s+exists`, err.Error())
			if !matches {
				return err
			}
		}
	}

	return nil
}

func (d *databaseConfig) connectionString(withDatabase bool) string {
	switch {
	case d.Adapter == "postgres":
		str := ""
		if len(d.User) > 0 {
			str += "user=" + d.User + " "
		}
		if len(d.Password) > 0 {
			str += "password=" + d.Password + " "
		}
		if withDatabase && len(d.DBName) > 0 {
			str += "dbname=" + d.DBName + " "
		}
		str += "sslmode="
		if len(d.SSLMode) > 0 {
			str += d.SSLMode + " "
		} else {
			str += "disable "
		}
		return str
	case d.Adapter == "sqlite3":
		return filepath.Join("data", d.DBName)
	case d.Adapter == "mysql":
		str := ""
		if len(d.User) > 0 {
			str += d.User
			if len(d.Password) > 0 {
				str += ":" + d.Password
			}
			str += "@"
		}
		str += "/"
		if withDatabase {
			str += d.DBName
		}

		return str
	}

	return ""
}

// Factory is an interface that defines how to create new references to the database.
type Factory interface {
	Open() (*gorm.DB, error)
	MustOpen() *gorm.DB
}

// DefaultFactory is the factory that should be used to generate database
// connections.
var DefaultFactory Factory = &ConfigFactory{
	initialized: false,
}
