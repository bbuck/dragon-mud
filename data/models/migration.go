package models

import "github.com/bbuck/dragon-mud/data"

var haveMigrated = false

// MigrateDatabase will perform the necessary database migrations to configure
// and/or update the database with the appropriate values.
func MigrateDatabase() error {
	if haveMigrated {
		return nil
	}
	haveMigrated = true
	db, err := data.DefaultFactory.Open()
	if err != nil {
		return err
	}

	// Perform migrations below
	db.AutoMigrate(&Player{})

	return nil
}
