package talon

import (
	"encoding/json"
	"regexp"
)

func init() {
	Unmarshalers["J"] = func(bs []byte) (interface{}, error) {
		j := &JSON{}
		err := j.UnmarshalTalon(bs)
		if err != nil {
			return nil, err
		}

		return j.Data, nil
	}
}

var jsonRx = regexp.MustCompile(`^J!(.+)$`)

// JSONParseError represents an error parsing a JSON value stored in Neo4j
type JSONParseError string

// Error allows JSONParseError to match the error interface.
func (jpe JSONParseError) Error() string {
	return string(jpe)
}

// JSON wraps an interface to provide a means for impelmenting the talon
// marshaling library.
type JSON struct {
	Data interface{}
}

// NewJSON returns a pointer to the JSON type as a helper.
func NewJSON(i interface{}) *JSON {
	return &JSON{
		Data: i,
	}
}

// Map returns the data (or nil) that is contained within this JSON value as
// a map[string]interface{}
func (j *JSON) Map() map[string]interface{} {
	if m, ok := j.Data.(map[string]interface{}); ok {
		return m
	}

	return nil
}

// Slice returns the data (or nil) that is contained within this JSON value as
// a []interface{}
func (j *JSON) Slice() []interface{} {
	if s, ok := j.Data.([]interface{}); ok {
		return s
	}

	return nil
}

// MarshalTalon implements the talon.Marshaler interface for the JSON type.
func (j *JSON) MarshalTalon() ([]byte, error) {
	bs, err := json.Marshal(j.Data)

	bs = append([]byte{'J', '!'}, bs...)

	return bs, err
}

// UnmarshalTalon converts the JSON string back into a go type
func (j *JSON) UnmarshalTalon(bs []byte) error {
	vals := jsonRx.FindAllSubmatch(bs, 1)

	if len(vals) == 0 {
		return JSONParseError("JSON string missing correct format")
	}

	return json.Unmarshal(vals[0][1], &j.Data)
}
