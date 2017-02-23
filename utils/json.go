// Copyright (c) 2016-2017 Brandon Buck

package utils

import (
	"bytes"
	"encoding/json"

	"github.com/bbuck/dragon-mud/logger"
	"github.com/fatih/structs"
)

// ToJSON take the object given and return a JSON string representation of that
// object.
func ToJSON(i interface{}) string {
	buf := new(bytes.Buffer)
	encoder := json.NewEncoder(buf)
	if err := encoder.Encode(i); err != nil {
		logger.NewWithSource("json_util").WithError(err).Error("Failed to encode object to JSON")

		return ""
	}

	return buf.String()
}

func ToMap(i interface{}) map[string]interface{} {
	return structs.Map(i)
}
