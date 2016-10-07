package utils

import (
	"bytes"
	"encoding/json"

	"github.com/bbuck/dragon-mud/logger"
)

func ToJSON(i interface{}) string {
	buf := new(bytes.Buffer)
	encoder := json.NewEncoder(buf)
	if err := encoder.Encode(i); err != nil {
		logger.WithField("error", err.Error()).Error("Failed to encode object to JSON")

		return ""
	}

	return buf.String()
}
