// Copyright (c) 2016 Brandon Buck

package talon

import (
	"errors"

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

type Row []Entity

type Rows struct {
	Metadata *Metadata
	Columns  []string

	boltRows bolt.Rows
}

func wrapBoltRows(rs bolt.Rows) *Rows {
	return &Rows{
		Metadata: metadataFromBoltRows(rs),
		boltRows: rs,
	}
}

func (r *Rows) Close() {
	r.boltRows.Close()
}

func (r *Rows) Next() (Row, error) {
	boltRow, _, err := r.boltRows.NextNeo()
	row := make(Row, len(boltRow))
	for i := 0; i < len(row); i++ {
		if node, ok := boltRow[i].(boltGraph.Node); ok {
			row[i] = wrapBoltNode(node)
		} else if rel, ok := boltRow[i].(boltGraph.Relationship); ok {
			row[i] = wrapBoltRelationship(rel)
		} else {
			// TODO: Remove
			panic(errors.New("this doesn't happen!!!"))
		}
	}

	return row, err
}

func (r *Rows) All() ([][]interface{}, map[string]interface{}, error) {
	return r.boltRows.All()
}
