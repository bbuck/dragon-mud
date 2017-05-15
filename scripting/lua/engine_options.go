package lua

// NamingConvention defines how Go names should be converted into the Lua.
type NamingConvention int8

const (
	// SnakeCaseExportedNames converts all Go names to both their snake_case
	// and Exported styles (ex for 'HelloWorld' you get 'hello_world' and
	// 'HelloWorld')
	SnakeCaseExportedNames NamingConvention = iota

	// SnakeCaseNames converts Go names into snake_case only.
	SnakeCaseNames

	// ExportedNames converts Go names into Go-exported type case normally
	// (essentially meaning the exported name is unchanged when transitioning
	// to Lua)
	ExportedNames
)

// EngineOptions allows for customization of a lua.Engine such as altering
// the names of fields and methods as well as whether or not to open all
// libraries.
type EngineOptions struct {
	OpenLibs     bool
	FieldNaming  NamingConvention
	MethodNaming NamingConvention
}
