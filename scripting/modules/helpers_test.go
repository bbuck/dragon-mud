package modules_test

import "github.com/bbuck/dragon-mud/scripting/lua"

func testReturn(eng *lua.Engine, script string) ([]*lua.Value, error) {
	n := eng.StackSize()
	err := eng.DoString(script)
	if err != nil {
		return []*lua.Value{eng.Nil()}, err
	}

	diff := eng.StackSize() - n
	var results []*lua.Value
	for i := 0; i < diff; i++ {
		val := eng.PopValue()
		results = append(results, val)
	}

	return results, nil
}
