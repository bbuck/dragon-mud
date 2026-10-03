package main

import (
	"slices"
	"testing"
)

func TestPluginSourcesLoadsListedBuiltinsInEngineOrder(t *testing.T) {
	sources, err := pluginSources(t.TempDir(), []string{"presence", "chat"})
	if err != nil {
		t.Fatal(err)
	}

	var got []string
	for _, s := range sources {
		got = append(got, s.Origin)
	}
	if want := []string{"built-in plugin chat", "built-in plugin presence"}; !slices.Equal(got, want) {
		t.Errorf("sources = %q, want %q", got, want)
	}
}
