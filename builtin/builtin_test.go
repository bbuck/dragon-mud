package builtin

import (
	"strings"
	"testing"

	"bbuck.dev/dragon-mud/plugin"
)

// The engine's APIs are the dragon: ones, so every API a built-in provides
// is in that namespace.
func TestBuiltinAPIsAreNamespaced(t *testing.T) {
	apis, err := APIs()
	if err != nil {
		t.Fatal(err)
	}
	if len(apis) == 0 {
		t.Fatal("no built-in provides an API; dragon:chat should")
	}
	for _, api := range apis {
		if !strings.HasPrefix(api, plugin.BuiltinPrefix) {
			t.Errorf("a built-in provides %q, want it named %s%s", api, plugin.BuiltinPrefix, api)
		}
	}
}
