package engine

// ScriptableObject defines an interface that returns an object to represent
// it in a script. For example, in game scripts should not have access to a full
// Player reference, but instead will receive a "scriptable player" reference that
// controls what can be done to a player. Most objects returned would be small
// wrapper objects.
type ScriptableObject interface {
	ScriptObject() interface{}
}
