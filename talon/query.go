// Copyright (c) 2016-2017 Brandon Buck

package talon

import (
	bolt "github.com/johnnadratowski/golang-neo4j-bolt-driver"
)

var noProperties = make(Properties)

// Query reprsents a Talon query before it's been converted in Cypher
type Query struct {
	db         *DB
	rawCypher  string
	properties Properties
}

// ToCypher converts a query object into a Cypher query string.
// NOTE: For the time being raw queries (strings with property injection)
//       are the only types of queries supported.
func (q *Query) ToCypher() string {
	if q.rawCypher != "" {
		return q.rawCypher
	}

	return "__INVALID__;"
}

// Query executes a fetch query, expecting rows to be returned.
func (q *Query) Query() (*Rows, error) {
	conn, stmt, err := q.getStatement()
	if err != nil {
		return nil, err
	}

	rows, err := stmt.QueryNeo(q.propsForQuery())
	if err != nil {
		conn.Close()

		return nil, err
	}

	r := wrapBoltRows(rows)

	return r, nil
}

// Exec runs a query that doesn't expect rows to be returned.
func (q *Query) Exec() (*Result, error) {
	_, stmt, err := q.getStatement()
	if err != nil {
		return nil, err
	}

	result, err := stmt.ExecNeo(q.propsForQuery())
	if err != nil {
		return nil, err
	}

	return wrapBoltResult(result), nil
}

func (q *Query) getStatement() (bolt.Conn, bolt.Stmt, error) {
	conn, err := q.db.conn()
	if err != nil {
		return nil, nil, err
	}

	stmt, err := conn.PrepareNeo(q.ToCypher())
	if err != nil {
		conn.Close()

		return nil, nil, err
	}

	return conn, stmt, nil
}

func (q *Query) propsForQuery() map[string]interface{} {
	if len(q.properties) == 0 {
		return nil
	}

	return map[string]interface{}(q.properties)
}
