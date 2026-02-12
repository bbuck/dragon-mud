package scripting

import (
	"fmt"
)

type LuaValue struct {
	value any
}

func NewLuaValue(value any) Value {
	return &LuaValue{
		value: value,
	}
}

func (lv *LuaValue) IsNumber() bool {
	switch lv.value.(type) {
	case int, int8, int16, int32, int64, float32, float64:
		return true
	default:
		return false
	}
}

func (lv *LuaValue) IsString() bool {
	_, ok := lv.value.(string)

	return ok
}

func (lv *LuaValue) ToBool() bool {
	if lv.value == nil {
		return false
	}

	if b, ok := lv.value.(bool); ok {
		return b
	}

	// any non-nil, non-bool value is true
	return true
}

func (lv *LuaValue) ToNumber() (float64, bool) {
	switch v := lv.value.(type) {
	case int:
		return float64(v), true
	case int8:
		return float64(v), true
	case int16:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case float32:
		return float64(v), true
	case float64:
		return float64(v), true
	default:
		return 0, false
	}
}

func (lv *LuaValue) ToString() (string, bool) {
	switch value := lv.value.(type) {
	case fmt.Stringer:
		return value.String(), true
	case string:
		return value, true
	default:
		return "", false
	}
}
