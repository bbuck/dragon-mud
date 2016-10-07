package data

import (
	"errors"
	"fmt"

	"github.com/bbuck/dragon-mud/logger"
	neo4j "github.com/johnnadratowski/golang-neo4j-bolt-driver"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
)

// ConfigFactory generates a gorm database connection based on configuration
// settings.
type ConfigFactory struct {
	initialized bool
	db          neo4j.DriverPool
}

// Open will load in configuration from the config file and generate a gorm
// connection.
func (cf ConfigFactory) Open() (DB, error) {
	if !cf.initialized {
		cf.initialized = true
		configKey := fmt.Sprintf("database.%s", viper.GetString("env"))
		configMap := viper.GetStringMapString(configKey)
		if configMap == nil {
			return nil, errors.New("No database configuration for environemnt")
		}
		config := new(databaseConfig)
		if err := mapstructure.Decode(configMap, &config); err != nil {
			return nil, err
		}
		if !config.valid() {
			return nil, errors.New("Invalid database configuration, make sure you have 'adapter' and 'dbname' set")
		}
		db, err := neo4j.NewDriverPool(config.connectionString(), config.ConnectionMax)
		if err != nil {
			return nil, err
		}
		cf.db = db
	}

	return cf.db.OpenPool()
}

// MustOpen fetches a reference to the shared database connection object. It's
// shorthand for calling DefaultFactory.Open() and handling an error
func (cf ConfigFactory) MustOpen() DB {
	db, err := DefaultFactory.Open()
	if err != nil {
		logger.WithField("error", err.Error()).Fatal("Failed to fetch cached DB connection")
	}

	return db
}
