// Copyright (c) 2016-2017 Brandon Buck

package talon

import (
	"fmt"

	bolt "github.com/johnnadratowski/golang-neo4j-bolt-driver"
	boltGraph "github.com/johnnadratowski/golang-neo4j-bolt-driver/structures/graph"
)

// &{
//   metadata: map[
//     fields:[n]
//   ]
//   statement: 0xc420430000
//   closed: false
//   consumed: true
//   finishedConsume: false
//   pipelineIndex: 0
//   closeStatement: true
// }

// Metadata contains details about the rows response, such as the field names
// from the query.
type Metadata struct {
	Fields []string
}

func metadataFromBoltRows(rows bolt.Rows) *Metadata {
	md := rows.Metadata()
	mdFields := md["fields"].([]interface{})
	fields := make([]string, len(mdFields))
	for i := 0; i < len(fields); i++ {
		fields[i] = mdFields[i].(string)
	}

	return &Metadata{
		Fields: fields,
	}
}

// Row represents a list of graph entities.
type Row []Entity

// Rows represents a group of rows fetched from a Cypher query.
type Rows struct {
	Metadata *Metadata
	Columns  []string

	closed   bool
	boltRows bolt.Rows
}

// create a talon.Rows object from a bolt.Rows object.
func wrapBoltRows(rs bolt.Rows) *Rows {
	return &Rows{
		Metadata: metadataFromBoltRows(rs),
		boltRows: rs,
	}
}

// Close will close the incoming stream of graph entities.
func (r *Rows) Close() {
	if !r.closed {
		r.closed = true
		r.boltRows.Close()
	}
}

// Next fetches the next row in the resultset.
func (r *Rows) Next() (Row, error) {
	boltRow, _, err := r.boltRows.NextNeo()
	row := make(Row, len(boltRow))
	for i, boltEnt := range boltRow {
		row[i] = boltToTalonEntity(boltEnt)
	}

	return row, err
}

// All returns all the rows up front instead of using the streaming API.
func (r *Rows) All() ([]Row, error) {
	all, _, err := r.boltRows.All()
	if err != nil {
		return nil, err
	}
	results := make([]Row, 0)
	for _, boltRow := range all {
		row := make(Row, len(boltRow))
		for i, boltEnt := range boltRow {
			row[i] = boltToTalonEntity(boltEnt)
		}
		results = append(results, row)
	}
	r.Close()

	return results, nil
}

// bolt type to talon type
func boltToTalonEntity(i interface{}) Entity {
	switch e := i.(type) {
	case boltGraph.Node:
		return wrapBoltNode(e)
	case boltGraph.Relationship:
		return wrapBoltRelationship(e)
	default:
		panic(fmt.Errorf("found %T value, didn't expect it", i))
	}

	return nil
}
