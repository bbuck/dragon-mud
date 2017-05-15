package data

import (
	"fmt"

	"github.com/bbuck/dragon-mud/logger"
	"github.com/bbuck/dragon-mud/talon"
	"github.com/spf13/viper"
)

var (
	connectOptions *talon.ConnectOptions
	db             *talon.DB
	log            logger.Log
)

// DB fetches a connection to the database using the connection information
// provided in the Dragonfile for the current environment.
func DB() *talon.DB {
	if connectOptions == nil {
		env := viper.GetString("env")
		log = logger.NewWithSource("database").WithField("env", env)
		log.Debug("Connecting to database for environment.")

		connectOptions = &talon.ConnectOptions{
			User: viper.GetString(fmt.Sprintf("database.%s.username", env)),
			Host: viper.GetString(fmt.Sprintf("database.%s.host", env)),
			Port: uint16(viper.GetInt64(fmt.Sprintf("database.%s.port", env))),
			Pool: uint16(viper.GetInt64(fmt.Sprintf("database.%s.connection_max", env))),
		}

		if viper.GetBool(fmt.Sprintf("database.%s.authentication", env)) {
			connectOptions.Pass = viper.GetString(fmt.Sprintf("database.%s.password", env))
		}

		var err error
		db, err = connectOptions.Connect()
		if err != nil {
			log.WithError(err).Fatal("Failed to initialize connection with database.")
		}
		log.Debug("Successfully connected to the database.")
	}

	return db
}
