// Copyright (c) 2016-2017 Brandon Buck

package talon

import (
	"time"

	bolt "github.com/johnnadratowski/golang-neo4j-bolt-driver"

	"github.com/bbuck/dragon-mud/talon/types"
)

// DB represents a talon connection to a Neo4j database using Neo4j bolt behind
// the scenes.
type DB struct {
	driver Driver
}

// CypherP performs the same job as Cypher, it just allows the user to pass in
// a set of properties.
func (d *DB) CypherP(cypher string, p types.Properties) (*Query, error) {
	props, err := talonMarshalProperties(p)
	if err != nil {
		return nil, err
	}

	q := &Query{
		db:         d,
		rawCypher:  cypher,
		properties: props,
	}

	return q, nil
}

// MustCypherP calls MustCypher but will panic on error.
func (d *DB) MustCypherP(cypher string, p types.Properties) *Query {
	q, err := d.CypherP(cypher, p)
	if err != nil {
		panic(err)
	}

	return q
}

// Cypher returns a query read to run on Neo4j from a raw Cypher string. This
// method assumes there are no properties to be added to the query.
func (d *DB) Cypher(cypher string) *Query {
	q, _ := d.CypherP(cypher, noProperties)

	return q
}

func (d *DB) conn() (bolt.Conn, error) {
	c, err := d.driver.Conn()

	return c, err
}

func talonMarshalProperties(p types.Properties) (types.Properties, error) {
	mp := make(types.Properties)
	for k, v := range p {
		switch t := v.(type) {
		case types.Marshaler:
			bs, err := t.MarshalTalon()
			if err != nil {
				return mp, err
			}
			mp[k] = string(bs)
		case complex128, complex64:
			bs, err := types.NewComplex(t).MarshalTalon()
			if err != nil {
				return nil, err
			}
			mp[k] = string(bs)
		case time.Time:
			bs, err := types.NewTime(t).MarshalTalon()
			if err != nil {
				return nil, err
			}
			mp[k] = string(bs)
		default:
			mp[k] = v
		}
	}

	return mp, nil
}
