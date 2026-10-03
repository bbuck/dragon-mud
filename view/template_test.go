package view

import (
	"strings"
	"testing"
)

func add(t *testing.T, ts *Templates, name, format, plugin, source string) {
	t.Helper()

	err := ts.Add(File{
		Name:   name,
		Format: format,
		Path:   plugin + "/views/" + name + "." + format + ".tmpl",
		Plugin: plugin,
		Source: source,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func render(t *testing.T, ts *Templates, name, format, block string, data any) string {
	t.Helper()

	out, ok, err := ts.Render(name, format, block, data)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatalf("no %s template for %s", format, name)
	}

	return out
}

var hit = `
{{define "actor"}}You hit {{entity .target}}.{{end}}
{{define "others"}}
{{entity .actor}} hits {{entity .target}}.
{{end}}
`

func TestRenderWholeTemplateAndBlocks(t *testing.T) {
	ts := NewTemplates()
	add(t, ts, "ambient", FormatText, "game", "[c]{{.text}}[x]\n")
	add(t, ts, "hit", FormatText, "game", hit)

	bob := Entity{"id": "b0b", "name": "Bob"}
	goblin := Entity{"id": "g0b", "name": "goblin"}
	data := map[string]any{"actor": bob, "target": goblin}

	tests := []struct {
		name, kind, block string
		data              any
		want              string
	}{
		{"whole file, trimmed", "ambient", "", map[string]any{"text": "A wind blows."}, "[c]A wind blows.[x]"},
		{"block", "hit", "actor", data, "You hit goblin."},
		{"block on its own lines", "hit", "others", data, "Bob hits goblin."},
		{"whole file of blocks is empty", "hit", "", data, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := render(t, ts, tt.kind, FormatText, tt.block, tt.data); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRenderUnknownBlock(t *testing.T) {
	ts := NewTemplates()
	add(t, ts, "hit", FormatText, "dragon:combat", hit)
	add(t, ts, "ambient", FormatText, "game", "{{.text}}")

	_, _, err := ts.Render("hit", FormatText, "atcor", map[string]any{})
	want := `dragon:combat/views/hit.txt.tmpl has no block "atcor". It defines actor, others; add {{define "atcor"}}...{{end}} to it, or send one of those.`
	if err == nil || err.Error() != want {
		t.Errorf("got error %v,\nwant %s", err, want)
	}

	_, _, err = ts.Render("ambient", FormatText, "actor", map[string]any{})
	want = `game/views/ambient.txt.tmpl has no block "actor". It doesn't define any blocks; add {{define "actor"}}...{{end}} to it, or send one of those.`
	if err == nil || err.Error() != want {
		t.Errorf("got error %v,\nwant %s", err, want)
	}
}

func TestRenderHTML(t *testing.T) {
	ts := NewTemplates()
	add(t, ts, "say", FormatText, "dragon:chat", `{{entity .actor}} says, "{{.message}}"`)
	add(t, ts, "say", FormatHTML, "game", `<q>{{.message}}</q> says {{entity .actor}}`)

	data := map[string]any{
		"actor":   Entity{"id": "b0b", "name": "Bob <the brave>"},
		"message": "<script>",
	}

	got := render(t, ts, "say", FormatHTML, "", data)
	want := `<q>&lt;script&gt;</q> says <dragon-entity ref="b0b">Bob &lt;the brave&gt;</dragon-entity>`
	if got != want {
		t.Errorf("HTML:\n got %s\nwant %s", got, want)
	}

	// The game's HTML file didn't replace the plugin's text one.
	if got := render(t, ts, "say", FormatText, "", data); got != `Bob <the brave> says, "<script>"` {
		t.Errorf("text: got %q", got)
	}
}

func TestRenderHTMLFallsBackToText(t *testing.T) {
	ts := NewTemplates()
	add(t, ts, "hit", FormatText, "game", `
{{define "others"}}[r]{{entity .actor}} hits {{entity .target}}![x]
<{{.how}}>{{end}}
`)

	data := map[string]any{
		"actor":  Entity{"id": "b0b", "name": "Bob & co"},
		"target": Entity{"id": "g0b", "key": "goblin"},
		"how":    "hard",
	}

	got := render(t, ts, "hit", FormatHTML, "others", data)
	want := `<span class="ansi-fg-1"><dragon-entity ref="b0b">Bob &amp; co</dragon-entity> hits <dragon-entity ref="g0b">goblin</dragon-entity>!</span><br>&lt;hard&gt;`
	if got != want {
		t.Errorf("\n got %s\nwant %s", got, want)
	}

	// The plain text is unchanged by the HTML version.
	if got := render(t, ts, "hit", FormatText, "others", data); got != "[r]Bob & co hits goblin![x]\n<hard>" {
		t.Errorf("text: got %q", got)
	}
}

func TestRenderMissing(t *testing.T) {
	ts := NewTemplates()
	add(t, ts, "tip", FormatHTML, "game", "<b>tip</b>")

	if _, ok, _ := ts.Render("nope", FormatText, "", nil); ok {
		t.Error("rendered a template that doesn't exist")
	}
	if _, ok, _ := ts.Render("tip", FormatText, "", nil); ok {
		t.Error("an HTML-only template rendered as text")
	}
}

func TestEntityNeedsAnObject(t *testing.T) {
	ts := NewTemplates()
	add(t, ts, "hit", FormatText, "game", "{{entity .actr}}")

	_, _, err := ts.Render("hit", FormatText, "", map[string]any{"actor": Entity{"id": "b0b"}})
	if err == nil || !strings.Contains(err.Error(), "entity needs an object, but got nothing. Check that the data key is spelled the same") {
		t.Errorf("got %v", err)
	}
}

func TestEntityName(t *testing.T) {
	tests := []struct {
		e    Entity
		want string
	}{
		{Entity{"id": "1", "key": "tavern", "name": "The Tavern"}, "The Tavern"},
		{Entity{"id": "1", "key": "tavern"}, "tavern"},
		{Entity{"id": "1", "name": 42}, "something"},
		{Entity{"id": "1"}, "something"},
	}
	for _, tt := range tests {
		if got := tt.e.Name(); got != tt.want {
			t.Errorf("%v: got %q, want %q", tt.e, got, tt.want)
		}
	}
}

func TestValidate(t *testing.T) {
	ts := NewTemplates()
	add(t, ts, "say", FormatText, "game", "")
	add(t, ts, "say", FormatHTML, "game", "")
	if err := ts.Validate("views"); err != nil {
		t.Errorf("valid templates: %v", err)
	}

	add(t, ts, "hit", FormatHTML, "game", "")
	want := "game/views/hit.html.tmpl has no text version. Add views/hit.txt.tmpl: telnet shows it, and the web does whenever there's no HTML version."
	if err := ts.Validate("views"); err == nil || err.Error() != want {
		t.Errorf("got %v,\nwant %s", err, want)
	}
}

func TestLaterFilesReplaceEarlier(t *testing.T) {
	ts := NewTemplates()
	add(t, ts, "say", FormatText, "dragon:chat", "built-in")
	add(t, ts, "say", FormatText, "game", "game")

	if got := render(t, ts, "say", FormatText, "", nil); got != "game" {
		t.Errorf("got %q", got)
	}
	if !ts.Has("say") || ts.Has("hit") {
		t.Error("Has is wrong")
	}
}

func TestParseError(t *testing.T) {
	err := NewTemplates().Add(File{Name: "hit", Format: FormatText, Path: "game/views/hit.txt.tmpl", Source: "{{.actor"})
	if err == nil || !strings.Contains(err.Error(), "game/views/hit.txt.tmpl:1") {
		t.Errorf("got %v, want an error with the file and line", err)
	}
}
