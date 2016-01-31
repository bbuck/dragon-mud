package data

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/bbuck/dragon-mud/logger"
	"github.com/jinzhu/gorm"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"

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

// ConfigFactory generates a gorm database connection based on configuration
// settings.
type ConfigFactory struct {
	initialized bool
	db          gorm.DB
}

// Open will load in configuration from the config file and generate a gorm
// connection.
func (cf ConfigFactory) Open() (*gorm.DB, error) {
	if !cf.initialized {
		cf.initialized = true
		configKey := fmt.Sprintf("database.%s", viper.GetString("env"))
		configMap := viper.GetStringMapString(configKey)
		if configMap == nil {
			return &cf.db, errors.New("No database configuration for environemnt")
		}
		config := new(databaseConfig)
		if err := mapstructure.Decode(configMap, &config); err != nil {
			return &cf.db, err
		}
		if !config.valid() {
			return &cf.db, errors.New("Invalid database configuration, make sure you have 'adapter' and 'dbname' set")
		}
		if err := config.createDatabase(); err != nil {
			return &cf.db, err
		}
		db, err := gorm.Open(config.Adapter, config.connectionString(true))
		if err != nil {
			return &cf.db, err
		}
		// Set configuration for database
		if config.Adapter == "mysql" {
			cf.db.Set("gorm:table_options", "ENGINE=InnoDB")
		}
		cf.db = db
	}

	return &cf.db, nil
}

// MustOpen fetches a reference to the shared database connection object. It's
// shorthand for calling DefaultFactory.Open() and handling an error
func (cf ConfigFactory) MustOpen() *gorm.DB {
	db, err := DefaultFactory.Open()
	if err != nil {
		logger.Log().WithField("error", err.Error()).Fatal("Failed to fetch cached DB connection")
	}

	return db
}
