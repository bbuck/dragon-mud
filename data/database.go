package data

import "github.com/jinzhu/gorm"

type databaseConfig struct {
	Adapter                string
	File                   string
	DSN                    string
	User, Password, DBName string
	SSLMode                string
}

// Factory is an interface that defines how to create new references to the database.
type Factory interface {
	Open() (*gorm.DB, error)
}

type ConfigFactory struct{}

func (cf ConfigFactory) Open() (*gorm.DB, error) {

}
