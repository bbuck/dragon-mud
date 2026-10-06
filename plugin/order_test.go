package plugin

import (
	"testing"
	"testing/fstest"
)

func source(name, manifest string) Source {
	return Source{Origin: name, Files: fstest.MapFS{
		ManifestFile: {Data: []byte("name = \"" + name + "\"\n" + manifest)},
	}}
}

func TestOrder(t *testing.T) {
	sources := []Source{
		source("combat", "[depends]\n\"johns:skills\" = \"^1.0\"\n\"johns:dice\" = \"^1.0\""),
		source("dice", "[provides]\n\"johns:dice\" = \"1.0\""),
		source("mapping", ""),
		source("skills", "[provides]\n\"johns:skills\" = \"1.0\"\n[depends]\n\"johns:dice\" = \"^1.0\""),
		source("weather", "[depends]\n\"skywatch:clouds\" = { version = \"^1.0\", optional = true }"),
	}

	sorted := Order(sources)
	var got []string
	for _, s := range sorted {
		got = append(got, s.Origin)
	}
	want := []string{"dice", "skills", "combat", "mapping", "weather"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}
