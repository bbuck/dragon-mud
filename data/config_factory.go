package data

import (
	"errors"
	"fmt"

	"github.com/bbuck/dragon-mud/logger"
	"github.com/jinzhu/gorm"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
)

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
		logger.WithField("error", err.Error()).Fatal("Failed to fetch cached DB connection")
	}

	return db
}
