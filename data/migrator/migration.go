package migrator

var haveMigrated = false

// MigrateDatabase will perform the necessary database migrations to configure
// and/or update the database with the appropriate values.
func MigrateDatabase() error {
	if haveMigrated {
		return nil
	}
	haveMigrated = true

	// db := data.DefaultFactory.MustOpen()

	// TODO: Execute queries to configure indexes and constraints.

	return nil
}
