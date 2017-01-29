// Copyright (c) 2016 Brandon Buck

package talon

import bolt "github.com/johnnadratowski/golang-neo4j-bolt-driver"

// ResultStats are some details about the results of the query, such as the
// number of of nodes, labels and properties added/set.
type ResultStats struct {
	LabelsAdded          int64
	NodesCreated         int64
	PropertiesSet        int64
	NodesDeleted         int64
	RelationshipsCreated int64
	RelationshipsDeleted int64
}

// Result represents a return from a non-row based query like a create/delete
// or upate where you're not using something like "return" in the query.
type Result struct {
	Stats ResultStats
	Type  string
}

// Close exits primarly to match Rows return behavior. When you run `Query`
// you have to close the rows object yourself. This is just here to prevent
// breakages.
func Close() { /* noop */ }

func wrapBoltResult(r bolt.Result) *Result {
	md := r.Metadata()
	res := &Result{
		Type:  maybeFetchString(md, "type"),
		Stats: ResultStats{},
	}

	if stats, ok := md["stats"].(map[string]interface{}); ok {
		res.Stats.LabelsAdded = maybeFetchInt64(stats, "labels-added")
		res.Stats.NodesCreated = maybeFetchInt64(stats, "nodes-created")
		res.Stats.PropertiesSet = maybeFetchInt64(stats, "properties-set")
		res.Stats.NodesDeleted = maybeFetchInt64(stats, "nodes-deleted")
		res.Stats.RelationshipsCreated = maybeFetchInt64(stats, "relationships-created")
		res.Stats.RelationshipsDeleted = maybeFetchInt64(stats, "relationships-deleted")
	}

	return res
}

func maybeFetchInt64(m map[string]interface{}, k string) int64 {
	if val, ok := m[k]; ok {
		var i64 int64
		if i64, ok = val.(int64); ok {
			return i64
		}
	}

	return 0
}

func maybeFetchString(m map[string]interface{}, k string) string {
	if val, ok := m[k]; ok {
		var str string
		if str, ok = val.(string); ok {
			return str
		}
	}

	return ""
}
