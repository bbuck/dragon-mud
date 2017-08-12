package talon

import (
	"bytes"
	"strings"
)

// CreateIndex is a nice way to query the database to create indexes instead
// of generating the query necessary to create an index.
func (db *DB) CreateIndex(label string, props []string) error {
	buf := new(bytes.Buffer)
	buf.WriteString("CREATE INDEX ON :")
	buf.WriteString(label)
	buf.WriteRune('(')
	buf.WriteString(strings.Join(props, ", "))
	buf.WriteRune(')')

	_, err := db.Cypher(buf.String()).Exec()

	return err
}
