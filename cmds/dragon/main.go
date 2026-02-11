package main

import (
	"bbuck.dev/dragon-mud/scripting"
	"github.com/Shopify/go-lua"
)

func main() {
	state := lua.NewState()
	lua.OpenLibraries(state)

	state.Register("add", func(l *lua.State) int {
		// TODO panic if we didn't get two arguments
		a, _ := l.ToNumber(1)
		b, _ := l.ToNumber(2)
		l.Pop(2)

		result := add(int(a), int(b))

		l.PushNumber(float64(result))

		return 1
	})

	err := lua.DoString(
		state,
		`
			result = add(1, 2)
			print('1 + 2 = ' .. result)
		`,
	)

	if err != nil {
		panic(err)
	}

	engine := scripting.ProvideEngine()

	engine.Register("add", add)

	err = engine.DoString(`
		result = add(1, 2)

		print('1 + 2 = ' .. result)
	`)

	if err != nil {
		panic(err)
	}
}

func add(a, b int) int {
	return a + b
}
