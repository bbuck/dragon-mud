// Copyright (c) 2016 Brandon Buck

package talon

import (
	"bytes"
	"fmt"

	bolt "github.com/johnnadratowski/golang-neo4j-bolt-driver"
)

// ConnectOptions allows customiztaino of how to connect to a Neo4j database
// with talon.
type ConnectOptions struct {
	User string
	Pass string
	Host string
	Port uint16
	Pool uint16
}

// URL takes the options set for connection and generates a bolt connection
// string.
func (co *ConnectOptions) URL() string {
	buf := new(bytes.Buffer)
	buf.WriteString("bolt://")
	if co.User != "" {
		buf.WriteString(co.User)
		if co.Pass != "" {
			buf.WriteRune(':')
			buf.WriteString(co.Pass)
		}
		buf.WriteRune('@')
	}
	buf.WriteString(co.Host)
	if co.Port > 0 {
		buf.WriteRune(':')
		buf.WriteString(fmt.Sprintf("%d", co.Port))
	}

	return buf.String()
}

// Connect will take the provided connection options and attempt to establish
// a connection to a Neo4j database.
func (co ConnectOptions) Connect() (db *DB, err error) {
	db = new(DB)
	if co.Pool == 0 {
		db.driver = &driver{
			Driver:         bolt.NewDriver(),
			connectOptions: co,
		}
	} else {
		var pool bolt.DriverPool
		pool, err = bolt.NewDriverPool(co.URL(), int(co.Pool))
		db.driver = &driverPool{
			pool: pool,
		}
	}

	return
}
