package scripting

import (
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

	// TODO: let's handle variadic functions seperately

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
			case reflect.Float32:
				num, ok := getNumberValue[float32](state, luaIndex)
				if !ok {
					failArgumentCount(state, luaIndex, "number")

					return 0
				}
				args = append(args, reflect.ValueOf(num))

			case reflect.Float64:
				num, ok := getNumberValue[float64](state, luaIndex)
				if !ok {
					failArgumentCount(state, luaIndex, "number")

					return 0
				}
				args = append(args, reflect.ValueOf(num))

			case reflect.Int:
				num, ok := getNumberValue[int](state, luaIndex)
				if !ok {
					failArgumentCount(state, luaIndex, "number")

					return 0
				}
				args = append(args, reflect.ValueOf(num))

			case reflect.Int8:
				num, ok := getNumberValue[int8](state, luaIndex)
				if !ok {
					failArgumentCount(state, luaIndex, "number")

					return 0
				}
				args = append(args, reflect.ValueOf(num))

			case reflect.Int16:
				num, ok := getNumberValue[int16](state, luaIndex)
				if !ok {
					failArgumentCount(state, luaIndex, "number")

					return 0
				}
				args = append(args, reflect.ValueOf(num))

			case reflect.Int32:
				num, ok := getNumberValue[int32](state, luaIndex)
				if !ok {
					failArgumentCount(state, luaIndex, "number")

					return 0
				}
				args = append(args, reflect.ValueOf(num))

			case reflect.Int64:
				num, ok := getNumberValue[int32](state, luaIndex)
				if !ok {
					failArgumentCount(state, luaIndex, "number")

					return 0
				}
				args = append(args, reflect.ValueOf(num))
			default:
				state.PushString(fmt.Sprintf("Unsupported argument type %s", inputType))
				state.Error()

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
