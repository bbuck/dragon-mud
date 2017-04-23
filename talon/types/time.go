// Copyright (c) 2016-2017 Brandon Buck

package types

import (
	"fmt"
	"regexp"
	"time"
)

// utility maps that allow easy conversion between a name, like ANSIC and the
// constant format value defined in the time package.
var timeNameToFormat = map[string]string{
	"ANSIC":       time.ANSIC,
	"UnixDate":    time.UnixDate,
	"RubyDate":    time.RubyDate,
	"RFC822":      time.RFC822,
	"RFC822Z":     time.RFC822Z,
	"RFC850":      time.RFC850,
	"RFC1123":     time.RFC1123,
	"RFC1123Z":    time.RFC1123Z,
	"RFC3339":     time.RFC3339,
	"RFC3339Nano": time.RFC3339Nano,
	"Kitchen":     time.Kitchen,
	"Stamp":       time.Stamp,
	"StampMilli":  time.StampMilli,
	"StampMicro":  time.StampMicro,
	"StampNano":   time.StampNano,
}

// assinged in init, the reverse of the above
var timeFormatToName map[string]string

func init() {
	Unmarshalers["T"] = func() Unmarshaler {
		t := EmptyTime()

		return &t
	}

	timeFormatToName = make(map[string]string)
	for n, f := range timeNameToFormat {
		timeFormatToName[f] = n
	}
}

// format for the final value after serialization, the first string is the
// format used to generate the output string, the second string is the formatted
// time value being stored
const timeSerializedFormat = "T!%s!!%s"

// timeRx is the format expected to parse for time, the first group is the
// format for parsing, and the second is the actual time string.
var timeRx = regexp.MustCompile(`^T!(.+?)!!(.+?)$`)

// DefaultTimeFormat is set to RFC3339 (predefined format) for use in storing
// retreiving dates. This is a profile of IOS 8601 date transfer formats and
// should be a widely supported format.
const DefaultTimeFormat = time.RFC3339

// TimeParseError represents an issue parsing time values.
type TimeParseError string

// Error implements the error interface for TimeParseError
func (t TimeParseError) Error() string {
	return string(t)
}

// Time wraps the standard lib time.Time to provide marshaling capability with
// the Talon ORM. This includes a format string for the time.
type Time struct {
	time.Time
	OutputFormat string
}

// EmptyTime returns a time object with time value, useful for unmarshaling.
func EmptyTime() Time {
	return Time{
		OutputFormat: DefaultTimeFormat,
	}
}

// NewTime wraps the given time.Time in a talon Time object.
func NewTime(t time.Time) Time {
	return Time{
		Time:         t,
		OutputFormat: DefaultTimeFormat,
	}
}

// EmptyTimeWithFormat returns a new time with the format specified, usefule for
// unmarshaling.
func EmptyTimeWithFormat(f string) Time {
	return Time{
		OutputFormat: f,
	}
}

// NewTimeWithFormat performs the same operation as NewTime but assigns a custom
// output format to the struct.
func NewTimeWithFormat(t time.Time, f string) Time {
	tt := NewTime(t)
	tt.OutputFormat = f

	return tt
}

// MarshalTalon allows Time to implement the Talon Marshaler interface.
func (t Time) MarshalTalon() ([]byte, error) {
	format := t.OutputFormat
	if f, ok := timeFormatToName[format]; ok {
		format = f
	}

	tstr := fmt.Sprintf(timeSerializedFormat, format, t.Format(t.OutputFormat))

	return []byte(tstr), nil
}

// UnmarshalTalon allows Time to implement the Talan Unmarshaler interface.
func (t *Time) UnmarshalTalon(bs []byte) error {
	vals := timeRx.FindAllSubmatch(bs, 1)

	if len(vals) < 1 {
		return TimeParseError("time string is not the correct format")
	}

	format := string(vals[0][1])
	value := string(vals[0][2])

	if f, ok := timeNameToFormat[format]; ok {
		format = f
	}

	if t.OutputFormat != format {
		t.OutputFormat = format
	}

	pt, err := time.Parse(t.OutputFormat, value)
	if err != nil {
		return TimeParseError(err.Error())
	}

	t.Time = pt

	return nil
}
