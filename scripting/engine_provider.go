package scripting

func ProvideEngine() Engine {
	return NewLuaEngine()
}
