package models_test

import (
	"os"

	"github.com/jinzhu/gorm"
)

// TestFactory is a factory for testing database functions
type TestFactory struct {
	db          gorm.DB
	initialized bool
}

// Open opens a Sqlite database, fits TestFactory to factory interface
func (t *TestFactory) Open() (*gorm.DB, error) {
	if !t.initialized {
		t.initialized = true
		os.Create("test_db.db3")
		db, err := gorm.Open("sqlite3", "test_db.db3")
		if err != nil {
			return nil, err
		}
		t.db = db
	}

	return &t.db, nil
}

// MustOpen fits TestFactory to Factory interface
func (t *TestFactory) MustOpen() *gorm.DB {
	db, err := t.Open()
	if err != nil {
		panic(err)
	}

	return db
}

// Cleanup resets the TestFactory and removes it's database file.
func (t *TestFactory) Cleanup() {
	t.db.Close()
	t.initialized = false
	os.Remove("test_db.db3")
}
