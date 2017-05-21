package modules

import (
	"fmt"
	"math"
	"reflect"
	"strconv"
	"time"

	"github.com/bbuck/dragon-mud/scripting/lua"
)

// Time provides some basic time functions for creating a time in Lua.
var Time = lua.TableMap{
	"now": func() *instantValue {
		t := instantValue(time.Now())

		return &t
	},
	"parse": func() {},
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
	"from_unix": func() {},
}

type instantValue time.Time

func (iv *instantValue) Inspect() string {
	return iv.Format(time.RFC3339)
}

func (iv *instantValue) Format(fstr string) string {
	t := time.Time(*iv)

	return t.Format(fstr)
}

func (iv *instantValue) Unix() int64 {
	return time.Time(*iv).Unix()
}

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
	year := 0
	month := time.January
	day := 0
	hour := 0
	min := 0
	sec := 0
	nsec := 0
	loc := time.UTC

	if iy, ok := m["year"]; ok {
		year = toInt(iy)

		fmt.Printf("\n\n%T\n\n%+v\n\n%+v\n\n", iy, iy, toInt(iy))
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

var timeZoneCache = map[string]*time.Location{
	"UTC":   time.UTC,
	"Local": time.Local,
}

func loadLocation(str string) (*time.Location, error) {
	if l, ok := timeZoneCache[str]; ok {
		return l, nil
	}

	l, err := time.LoadLocation(str)
	if err != nil {
		return nil, err
	}

	timeZoneCache[str] = l

	return l, nil
}

func toMonth(i interface{}) time.Month {
	switch s := i.(type) {
	case string:
		if m, ok := monthMap[s]; ok {
			return m
		}
	case *string:
		if m, ok := monthMap[*s]; ok {
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
