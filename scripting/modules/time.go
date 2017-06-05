package modules

import (
	"fmt"
	"math"
	"reflect"
	"regexp"
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
//     minute: number = alias for min
//     sec: number = the second (0-59) of the instant; default: 0,
//     second: number = alias for sec
//     milli: number = the number of milliseconds (past the second) of the
//       instant; defualt 0
//     millisecond: number = alias for milli
//     nano: number = the number of nanoseconds (past the second) of the
//       instant; default: 0,
//     nanosecond: number = alias for nano
//     zone: string = the name of the time zone for the instant in anything
//       other than "UTC"; default: UTC
//   }
//   durationData: table = {
//     nanosecond: number = number of nanoseconds in this duration,
//     millisecond: number = number of milliseconds in this duration,
//     second: number = number of seconds in this duration,
//     minute: number = number of minutes in this duration,
//     hour: number = number of hours in this duration,
//     day: number = number of days in this duration (assumed 24 hours),
//     week: number = number of weeks in this duration (assumed 7 days),
//     month: number = number of months in this duration (assumed 30 days),
//     year: number = number of years in this duration (assumed 365 days)
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
//   duration(generator): number
//     @param generator: table(durationData) | number | string = either a table
//       defining the values in the duration, a string encoding of the duration
//       or a numeric value that is the duration.
//     this method generates a duration value, it's range is roughly -290..290
//     years. forcing beyond this boundary is undefined and should be avoided at
//     all costs.
//   duration_parts(duration): table
//     @param duration: number = the number of nanoseconds representing a period
//       an arbitrary passing of time with no start point
//     take a duration value and break it into a map containing the named
//     components, like a duration of "1w" would come back with {weeks = 1}.
//     given the nature of durations being numbers, if a generated duration has
//     overlapping periods you can expect to get different components back, for
//     example "8d" (8 days) = {weeks = 1, days = 1}
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
//     add(duration): time.Instant
//       @param duration: number = the number of nanonseconds that need to be
//         to the current instant.
//       add a fixed duration to the given date and return the results as a new
//       date value.
//     sub(duration): time.Instant
//       @param duration: number = the number of nanonseconds that need to be
//         to the current instant.
//       much the same as :add, however this method will negate the duration.
//     sub_date(oinstant): number
//       @param oinstant: time.Instant = the instant you wish to subtract from the
//         current instant.
//       returns the duration, or nanosecond difference, between the original
//       instant and the instant you're subtracting from.
//     is_before(oinstant): boolean
//       @param oinstant: time.Instant = the other instant you're comparing
//         this instant too.
//       returns true if the current instant occurred _before_ the other
//       instant.
//     is_after(oinstant): boolean
//       @param oinstant: time.Instant = the other instant you're comparing
//         this instant too.
//       the opposite of :is_before, this checks to see if this instant occurred
//       _after_ the other.
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
	"duration": func(eng *lua.Engine) int {
		if eng.StackSize() == 0 {
			eng.ArgumentError(1, "expected an argument, but received none")

			return 0
		}

		val := eng.PopValue()
		var dur float64
		switch {
		case val.IsNumber():
			dur = val.AsNumber()
		case val.IsTable():
			dur = durationFromMap(val.AsMapStringInterface())
		case val.IsString():
			dur = durationFromString(val.AsString())
		}

		eng.PushValue(float64(floatToDuration(dur)))

		return 1
	},
	"duration_parts": func(f float64) map[string]float64 {
		if f > math.MaxInt64 || f < math.MinInt64 {
			return map[string]float64{
				"out_of_range": f,
			}
		}

		dur := floatToDuration(f)
		durMap := make(map[string]float64)

		year := durationMap["year"]
		temp := dur / year
		dur %= year
		durMap["years"] = float64(temp)

		month := durationMap["month"]
		temp = dur / month
		dur %= month
		durMap["months"] = float64(temp)

		week := durationMap["week"]
		temp = dur / week
		dur %= week
		durMap["weeks"] = float64(temp)

		day := durationMap["day"]
		temp = dur / day
		dur %= day
		durMap["days"] = float64(temp)

		temp = dur / time.Hour
		dur %= time.Hour
		durMap["hours"] = float64(temp)

		temp = dur / time.Minute
		dur %= time.Minute
		durMap["minutes"] = float64(temp)

		temp = dur / time.Second
		dur %= time.Second
		durMap["seconds"] = float64(temp)

		temp = dur / time.Millisecond
		dur %= time.Millisecond
		durMap["milliseconds"] = float64(temp)

		durMap["nanoseconds"] = float64(dur)

		return durMap
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

// Add a duration period to the given date.
func (iv *instantValue) Add(duration float64) *instantValue {
	dur := floatToDuration(duration)
	t := time.Time(*iv)
	ot := t.Add(dur)
	oiv := instantValue(ot)

	return &oiv
}

// Sub a duration period to the given date.
func (iv *instantValue) Sub(duration float64) *instantValue {
	dur := floatToDuration(duration)
	t := time.Time(*iv)
	ot := t.Add(-dur)
	oiv := instantValue(ot)

	return &oiv
}

func (iv *instantValue) SubDate(oiv *instantValue) float64 {
	t := time.Time(*iv)
	ot := time.Time(*oiv)
	dur := t.Sub(ot)

	return float64(dur)
}

// IsBefore determines whether or not the given date occurs before the time
// this metohd is called on.
func (iv *instantValue) IsBefore(oiv *instantValue) bool {
	t := time.Time(*iv)
	ot := time.Time(*oiv)

	return t.Before(ot)
}

// IsAfter determine whether or not the given date occurse after the time this
// method is called on.
func (iv *instantValue) IsAfter(oiv *instantValue) bool {
	t := time.Time(*iv)
	ot := time.Time(*oiv)

	return t.After(ot)
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

	if imi, ok := m["minutes"]; ok {
		min = toInt(imi)
	}

	if is, ok := m["sec"]; ok {
		sec = toInt(is)
	}

	if is, ok := m["seconds"]; ok {
		sec = toInt(is)
	}

	if ins, ok := m["milli"]; ok {
		nsec += toInt(ins) * int(time.Millisecond)
	}

	if ins, ok := m["millisecond"]; ok {
		nsec += toInt(ins) * int(time.Millisecond)
	}

	if ins, ok := m["nano"]; ok {
		nsec += toInt(ins)
	}

	if ins, ok := m["nanoseconds"]; ok {
		nsec += toInt(ins)
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

// convert a float to a duration, doing bound checking
func floatToDuration(f float64) time.Duration {
	if f > math.MaxInt64 {
		f = math.MaxInt64
	} else if f < math.MinInt64 {
		f = math.MinInt64
	}

	return time.Duration(round(f))
}

var durationMap = map[string]time.Duration{
	"nanosecond":   time.Nanosecond,
	"nanoseconds":  time.Nanosecond,
	"ns":           time.Nanosecond,
	"millisecond":  time.Millisecond,
	"milliseconds": time.Millisecond,
	"ms":           time.Millisecond,
	"second":       time.Second,
	"seconds":      time.Second,
	"s":            time.Second,
	"minute":       time.Minute,
	"minutes":      time.Minute,
	"m":            time.Minute,
	"hour":         time.Hour,
	"hours":        time.Hour,
	"h":            time.Hour,
	"day":          time.Hour * 24,
	"days":         time.Hour * 24,
	"d":            time.Hour * 24,
	"week":         time.Hour * 24 * 7,
	"weeks":        time.Hour * 24 * 7,
	"w":            time.Hour * 24 * 7,
	"month":        time.Hour * 24 * 30,
	"months":       time.Hour * 24 * 30,
	"M":            time.Hour * 24 * 30,
	"year":         (time.Hour * 24 * 7 * 52) + (time.Hour * 24),
	"years":        (time.Hour * 24 * 7 * 52) + (time.Hour * 24),
	"y":            (time.Hour * 24 * 7 * 52) + (time.Hour * 24),
}

func durationFromMap(m map[string]interface{}) float64 {
	var duration float64

	for k, v := range m {
		if dur, ok := durationMap[k]; ok {
			if f, fok := v.(float64); fok {
				f = round(f)
				duration += float64(dur) * f
			}
		}
	}

	return duration
}

var durationStringRx = regexp.MustCompile(`(-?\d+)(ns|ms|s|m|h|d|w|M|y)`)

func durationFromString(s string) float64 {
	var duration float64

	matches := durationStringRx.FindAllStringSubmatch(s, -1)
	for _, match := range matches {
		if dur, ok := durationMap[match[2]]; ok {
			f, _ := strconv.ParseFloat(match[1], 64)
			duration += float64(dur) * f
		}
	}

	return duration
}
