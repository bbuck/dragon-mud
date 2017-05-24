package modules

import (
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/bbuck/dragon-mud/scripting/lua"
)

// Time provides some basic time functions for creating a time in Lua.
//   instantData: table = {
//     year: number = the year the instant is set in; default: current year,
//     month: string | number = the three letter name, full name or numeric
//       represenation of the month: "jan", "January", 1; default: January
//     day: number = the day in which the instant is to be set; default: 1,
//     hour: number = the hour (0-23) of the instant; default: 0,
//     min: number = the minute (0-59) of the instant; default: 0,
//     sec: number = the second (0-59) of the instant; default: 0,
//     nano: number = the number of nanoseconds (past the second) of the
//       instant; default: 0,
//     zone: string = the name of the time zone for the instant in anything
//       other than "UTC"; default: UTC
//   }
//   now(): time.Instant
//     returns an instant value representing the current time in the UTC time
//     time zone.
//   parse(format, date): time.Instant | nil
//     @param format: string = the format to be used to parse the given date
//       date string with. This format is based on the same format that Go's
//       time package uses, 'Mon Jan 2 15:04:05 -0700 MST 2006', you can use a
//       wide variety of this date to specify the format.
//     @param date: string = the stringified date value that is to be parsed
//       with the given format.
//     attempts to parse the date value with the given format and returns an
//     instant value based on the given date string value -- if the date fails
//     to parse then nil will be returned.
//   create(data): time.Instant
//     @param data: instantData = a table containing the values to construct the
//       instant value from. All keys are optional.
//     builds an instant with any given information using default fallbacks
//     and returns this new instant.
//   unix(timestamp): time.Instant
//     @param timestamp: number = a Unix timestamp value used to generate an
//       instant value for the given date based on the rules of unix timestamps.
//     this method generates a time.Instant value based on the give timestamp.
//   time.Instant
//     format(format): string
//       @param format: string = the format that will be used to produce a
//         string representation
//       formats the date according to the string value. Like time.parse this
//       method uses the Go base date 'Mon Jan 2 15:04:05 -0700 MST 2006' as a
//       means for defining the format
//     unix(): number
//       returns the unix timestamp value for the given instant in time.
//     in_zone(zone): time.Instant
//       @param zone: string = the name of the time zone to transition this
//         instant to.
//       this will create a new instant and return that value in the given time
//       zome.
//     zone(): string
//       return the name of the time zone (not 100% accurate) used for the
//       instant's current time zone. It's not super accurate becuase if you
//       use 'local' you get 'local' back.
//     inspect(): string
//       returns a string that represents a debug output, primarily for use in
//       the REPL.
var Time = lua.TableMap{
	"now": func() *instantValue {
		t := instantValue(time.Now().UTC())

		return &t
	},
	"parse": func(fmt, date string) *instantValue {
		t, err := time.Parse(fmt, date)
		if err != nil {
			return nil
		}
		iv := instantValue(t)

		return &iv
	},
	"create": func(engine *lua.Engine) int {
		if engine.StackSize() < 1 {
			engine.RaiseError("a map of date information is required")

			return 0
		}

		arg := engine.PopValue()
		m := arg.AsMapStringInterface()

		iv, err := instantFromMap(m)
		if err != nil {
			engine.RaiseError(err.Error())

			return 0
		}

		engine.PushValue(iv)

		return 1
	},
	"unix": func(ts int64) *instantValue {
		t := time.Unix(ts, 0).UTC()
		iv := instantValue(t)

		return &iv
	},
}

// instantValue represents a moment in time, by default without a time zone
// (technically _with_ one (UTC) but a standard one.
type instantValue time.Time

// Inspect prints a pretty format for a time for use in the REPL.
func (iv *instantValue) Inspect() string {
	return fmt.Sprintf("time.Instant(%q)", iv.Format(time.UnixDate))
}

// Format implements a method that allows a time to be formatted per the Go
// format behaviors.
func (iv *instantValue) Format(fstr string) string {
	t := time.Time(*iv)

	return t.Format(fstr)
}

// Unix returns the Unix epoch timestamp for the current date.
func (iv *instantValue) Unix() int64 {
	return time.Time(*iv).Unix()
}

// InZone returns a new time object in the specified time zone if the specified
// time zone exists. If it does not exist, then this will return nil.
func (iv *instantValue) InZone(tz string) *instantValue {
	t := time.Time(*iv)
	loc, err := loadLocation(tz)
	if err != nil {
		return nil
	}
	t = t.In(loc)
	niv := instantValue(t)

	return &niv
}

// Zone returns the string name of the location associated with the current
// time value.
func (iv *instantValue) Zone() string {
	t := time.Time(*iv)

	return t.Location().String()
}

// map 3-letter and full month names to `time.Month` values
var monthMap = map[string]time.Month{
	"jan":       time.January,
	"january":   time.January,
	"feb":       time.February,
	"february":  time.February,
	"mar":       time.March,
	"march":     time.March,
	"apr":       time.April,
	"april":     time.April,
	"may":       time.May,
	"jun":       time.June,
	"june":      time.June,
	"jul":       time.July,
	"july":      time.July,
	"aug":       time.August,
	"august":    time.August,
	"sep":       time.September,
	"september": time.September,
	"oct":       time.October,
	"october":   time.October,
	"nov":       time.November,
	"november":  time.November,
	"dec":       time.December,
	"december":  time.December,
}

func instantFromMap(m map[string]interface{}) (*instantValue, error) {
	year := time.Now().Year()
	month := time.January
	day := 1
	hour := 0
	min := 0
	sec := 0
	nsec := 0
	loc := time.UTC

	if iy, ok := m["year"]; ok {
		year = toInt(iy)
	}

	if im, ok := m["month"]; ok {
		month = toMonth(im)
	}

	if id, ok := m["day"]; ok {
		day = toInt(id)
	}

	if ih, ok := m["hour"]; ok {
		hour = toInt(ih)
	}

	if imi, ok := m["min"]; ok {
		min = toInt(imi)
	}

	if is, ok := m["sec"]; ok {
		sec = toInt(is)
	}

	if ins, ok := m["nano"]; ok {
		nsec = toInt(ins)
	}

	var err error
	if il, ok := m["zone"]; ok {
		if locStr, ok := il.(string); ok {
			loc, err = loadLocation(locStr)
			if err != nil {
				return nil, err
			}
		}
	}

	t := time.Date(year, month, day, hour, min, sec, nsec, loc)
	iv := instantValue(t)

	return &iv, nil
}

// cache `time.Location` values for faster repetitive lookup
var timeZoneCache = map[string]*time.Location{
	"utc":   time.UTC,
	"local": time.Local,
}

// loadLocation will attempt to find the location from the time package and
// cache it's value before returning it. It first looks up the value in it's
// cache.
func loadLocation(str string) (*time.Location, error) {
	key := strings.ToLower(str)
	if l, ok := timeZoneCache[key]; ok {
		return l, nil
	}

	l, err := time.LoadLocation(str)
	if err != nil {
		return nil, err
	}

	timeZoneCache[key] = l

	return l, nil
}

// converts a string/numeric value into a `time.Month` value.
func toMonth(i interface{}) time.Month {
	switch s := i.(type) {
	case string:
		if m, ok := monthMap[strings.ToLower(s)]; ok {
			return m
		}
	case *string:
		if m, ok := monthMap[strings.ToLower(*s)]; ok {
			return m
		}
	}

	n := toInt(i)
	mn := time.Month(n)
	if mn >= time.January && mn <= time.December {
		return mn
	}

	return time.January
}

// convert an int/float value into an `int` Go type, taking into consideration
// the size of `int` for the compiled platform.
func toInt(i interface{}) int {
	v := reflect.ValueOf(i)
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i64 := v.Int()
		if strconv.IntSize == 64 {
			return int(i64)
		}

		switch {
		case i64 > int64(math.MaxInt32):
			return int(math.MaxInt32)
		case i64 < int64(math.MinInt32):
			return int(math.MinInt32)
		default:
			return int(i64)
		}
	case reflect.Float32, reflect.Float64:
		f64 := v.Float()
		switch {
		case f64 > float64(math.MaxInt32):
			return int(math.MaxInt32)
		case f64 < float64(math.MinInt32):
			return int(math.MinInt32)
		default:
			return int(f64)
		}
	}

	return 0
}
