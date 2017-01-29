// Copyright (c) 2016 Brandon Buck

package talon

import (
	"github.com/bbuck/talon/types"

	bolt "github.com/johnnadratowski/golang-neo4j-bolt-driver"
)

// DB represents a talon connection to a Neo4j database using Neo4j bolt behind
// the scenes.
type DB struct {
	driver Driver
}

// CypherP performs the same job as Cypher, it just allows the user to pass in
// a set of properties.
func (d *DB) CypherP(q string, p types.Properties) *Query {
	return &Query{
		db:         d,
		rawCypher:  q,
		properties: p,
	}
}

// Cypher returns a query read to run on Neo4j from a raw Cypher string. This
// method assumes there are no properties to be added to the query.
func (d *DB) Cypher(q string) *Query {
	return d.CypherP(q, noProperties)
}

func (d *DB) conn() (bolt.Conn, error) {
	c, err := d.driver.Conn()

	return c, err
}
