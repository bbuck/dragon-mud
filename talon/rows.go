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
	Fields   []string
	fieldMap map[string]int
}

func metadataFromBoltRows(rows bolt.Rows) *Metadata {
	md := rows.Metadata()
	mdFields := md["fields"].([]interface{})
	fields := make([]string, len(mdFields))
	fieldMap := make(map[string]int)
	for i := 0; i < len(fields); i++ {
		label := mdFields[i].(string)
		fields[i] = label
		fieldMap[label] = i
	}

	return &Metadata{
		Fields:   fields,
		fieldMap: fieldMap,
	}
}

// Row represents a list of graph entities.
type Row struct {
	fields   []Entity
	Metadata *Metadata
}

// Len returns the number of fields contained in the row.
func (r *Row) Len() int {
	return len(r.fields)
}

// GetColumn fetchs a column by it's associated name, so if you return the name
// 'node' in your query, you can fetch the value for that column via
// GetColumn("node").
func (r *Row) GetColumn(label string) (Entity, bool) {
	if idx, ok := r.Metadata.fieldMap[label]; ok {
		return r.fields[idx], true
	}

	return nil, false
}

// GetIndex returns the column by index, along with a bool no whether the index
// existed.
func (r *Row) GetIndex(idx int) (Entity, bool) {
	if idx >= 0 && idx < len(r.fields) {
		return r.fields[idx], true
	}

	return nil, false
}

// Rows represents a group of rows fetched from a Cypher query.
type Rows struct {
	Metadata *Metadata

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
func (r *Rows) Next() (*Row, error) {
	boltRow, _, err := r.boltRows.NextNeo()
	row := &Row{
		fields:   make([]Entity, len(boltRow)),
		Metadata: r.Metadata,
	}
	for i, boltEnt := range boltRow {
		ent, err := boltToTalonEntity(boltEnt)
		if err != nil {
			return nil, err
		}
		row.fields[i] = ent
	}

	return row, err
}

// All returns all the rows up front instead of using the streaming API.
func (r *Rows) All() ([]*Row, error) {
	all, _, err := r.boltRows.All()
	if err != nil {
		return nil, err
	}
	results := make([]*Row, 0)
	for _, boltRow := range all {
		row := &Row{
			fields:   make([]Entity, len(boltRow)),
			Metadata: r.Metadata,
		}
		for i, boltEnt := range boltRow {
			ent, err := boltToTalonEntity(boltEnt)
			if err != nil {
				return nil, err
			}
			row.fields[i] = ent
		}
		results = append(results, row)
	}
	r.Close()

	return results, nil
}

// bolt type to talon type
func boltToTalonEntity(i interface{}) (Entity, error) {
	if i == nil {
		return &Nil{}, nil
	}

	switch e := i.(type) {
	case boltGraph.Node:
		return wrapBoltNode(e)
	case boltGraph.Relationship:
		return wrapBoltRelationship(e)
	case boltGraph.UnboundRelationship:
		return wrapBoltUnboundRelationship(e)
	case boltGraph.Path:
		return wrapBoltPath(e)
	case string:
		val, err := tryUnmarshalString(e)
		if err != nil {
			return nil, err
		}

		switch t := val.(type) {
		case string:
			str := String(t)

			return &str, nil
		case Entity:
			return t, nil
		// default case is just using the original string
		default:
			str := String(e)

			return &str, nil
		}
	case int64:
		i := Int(e)

		return &i, nil
	case float64:
		f := Float(e)

		return &f, nil
	case bool:
		b := Bool(e)

		return &b, nil
	}

	return nil, fmt.Errorf("found %T value, didn't expect it", i)
}
