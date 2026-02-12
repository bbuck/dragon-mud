package scripting

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/Shopify/go-lua"
)

type LuaEngine struct {
	state *lua.State
}

func NewLuaEngine() Engine {
	state := lua.NewState()
	lua.OpenLibraries(state)

	return &LuaEngine{
		state: state,
	}
}

func (le *LuaEngine) DoString(code string) error {
	return lua.DoString(le.state, code)
}

func (le *LuaEngine) Register(name string, fn any) error {
	value := reflect.ValueOf(fn)
	valueType := value.Type()

	if valueType.Kind() != reflect.Func {
		return fmt.Errorf("expected a function but received: %s", valueType.String())
	}

	// TODO: let's handle variadic functions later

	argCount := valueType.NumIn()
	var inputTypes []reflect.Type
	for i := range argCount {
		inputTypes = append(inputTypes, valueType.In(i))
	}

	returnCount := valueType.NumOut()
	var outputTypes []reflect.Type
	for i := range returnCount {
		outputTypes = append(outputTypes, valueType.Out(i))
	}

	registerFn := func(state *lua.State) int {
		luaArgCount := state.Top()

		if luaArgCount < len(inputTypes) {
			state.PushString(
				fmt.Sprintf(
					"Arguments missing %s expected %d argument(s) but received %d arguments",
					name,
					len(inputTypes),
					luaArgCount,
				),
			)

			state.Error()

			return 0
		}

		var args []reflect.Value
		for i, inputType := range inputTypes {
			luaIndex := i + 1

			switch inputType.Kind() {
			case reflect.Float32, reflect.Float64, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				num, ok := state.ToNumber(luaIndex)
				if !ok {
					le.Fail(fmt.Errorf("expected argument #%d to be a number", luaIndex))

					return 0
				}

				goValue := reflect.ValueOf(num).Convert(inputType)
				args = append(args, goValue)

			default:
				le.Fail(fmt.Errorf("unsupported argument type %s", inputType))

				return 0
			}
		}

		state.Pop(luaArgCount)

		results := value.Call(args)

		if len(results) == 0 {
			return 0
		}

		pushed := 0

		for _, result := range results {
			switch result.Kind() {
			case reflect.Float32:
				fallthrough
			case reflect.Float64:
				state.PushNumber(result.Float())
			case reflect.Int:
				fallthrough
			case reflect.Int8:
				fallthrough
			case reflect.Int16:
				fallthrough
			case reflect.Int32:
				fallthrough
			case reflect.Int64:
				i := result.Int()
				state.PushNumber(float64(i))
			default:
				state.PushFString("Unsupported return type %s", result.Kind())
				state.Error()

				return pushed
			}

			pushed++
		}

		return pushed
	}

	le.state.Register(name, registerFn)

	return nil
}

func (le *LuaEngine) Fail(err error) {
	le.state.PushString(err.Error())
	le.state.Error()
}

func (le *LuaEngine) pushNumber(value any) error {
	switch v := value.(type) {
	case int:
		le.state.PushInteger(v)
	case int8:
		le.state.PushInteger(int(v))
	case int16:
		le.state.PushInteger(int(v))
	case int32:
		le.state.PushInteger(int(v))
	case int64:
		le.state.PushNumber(float64(v))
	case float32:
		le.state.PushNumber(float64(v))
	case float64:
		le.state.PushNumber(v)
	default:
		return errors.New("invalid numeric type pushed to state")
	}

	return nil
}

type numeric interface {
	int | int8 | int16 | int32 | int64 | float32 | float64
}

func getNumber[TGoType numeric](engine *LuaEngine, index int) (TGoType, error) {
	if !engine.state.IsNumber(index) {
		return 0, fmt.Errorf("value at index %d is not an number", index)
	}

	value, _ := engine.state.ToNumber(index)

	return TGoType(value), nil
}

func failArgumentCount(state *lua.State, index int, expectedType string) {
	if !state.IsNumber(index) {
		state.PushString(
			fmt.Sprintf(
				"Argument #%d expected %s, but received %s",
				index,
				expectedType,
				state.TypeOf(index),
			),
		)

		state.Error()
	}
}

type number interface {
	float32 | float64 | int | int8 | int16 | int32 | int64
}

func getNumberValue[TNum number](state *lua.State, index int) (TNum, bool) {
	num, ok := state.ToNumber(index)

	if !ok {
		return 0, ok
	}

	return TNum(num), ok
}
