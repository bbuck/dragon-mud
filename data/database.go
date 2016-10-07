package data

import (
	"github.com/bbuck/dragon-mud/logger"
	"github.com/bbuck/dragon-mud/text/tmpl"

	neo4j "github.com/johnnadratowski/golang-neo4j-bolt-driver"
)

const (
	authConnStringTemplate   = "bolt://%{host}:%{port}"
	noAuthConnStringTemplate = "bolt://%{username}:%{password}@%{host}:%{port}"
)

type DB interface {
	Create(interface{}) error
	Save(interface{}) error
	Delete(interface{}) error
}

type databaseConfig struct {
	Authentication bool
	Username       string
	Password       string
	Host           string
	Port           int
	ConnectionMax  int
}

func (d *databaseConfig) valid() bool {
	if d.Authentication && len(d.Username) == 0 && len(d.Password) == 0 {
		return false
	}

	return true
}

func (d *databaseConfig) connectionString() string {
	data := map[string]interface{}{
		"host": d.Host,
		"port": d.Port,
	}

	tmplStr := noAuthConnStringTemplate
	if d.Authentication {
		tmplStr = authConnStringTemplate
		data["username"] = d.Username
		data["password"] = d.Password
	}

	if str, err := tmpl.RenderOnce(tmplStr, data); err == nil {
		return str
	} else {
		logger.WithField("error", err.Error()).Fatal("Failed to generate connection data.")
	}

	return ""
}

// Factory is an interface that defines how to create new references to the database.
type Factory interface {
	Open() (neo4j.Conn, error)
	MustOpen() neo4j.Conn
}

// DefaultFactory is the factory that should be used to generate database
// connections.
var DefaultFactory Factory = &ConfigFactory{
	initialized: false,
}
