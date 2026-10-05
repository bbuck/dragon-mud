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
	add(t, ts, "hit", FormatText, "game", "{{entity .actor.weapon}}")

	_, _, err := ts.Render("hit", FormatText, "", map[string]any{"actor": Entity{"id": "b0b"}})
	if err == nil || !strings.Contains(err.Error(), "entity needs an object, but got nothing. Check that the data key is spelled the same") {
		t.Errorf("got %v", err)
	}
}

func TestMissingData(t *testing.T) {
	cases := map[string]struct {
		source string
		data   map[string]any
		want   string // empty when it should render
	}{
		"typo": {
			source: "{{entity .actr}}",
			data:   map[string]any{"actor": Entity{"id": "b0b"}},
			want:   "game/views/v.txt.tmpl:1:9: .actr isn't in the data. The data has actor. Send actr with the view, check the template for a typo, or wrap it in {{if .actr}}...{{end}} if it's optional.",
		},
		"stale data shape": {
			source: "<{{.room.name}}>",
			data:   map[string]any{"title": "Tavern", "exits": []any{}},
			want:   ".room isn't in the data (in .room.name). The data has exits, title.",
		},
		"empty data": {
			source: "{{$.room}}",
			data:   map[string]any{},
			want:   ".room isn't in the data. The data is empty.",
		},
		"missing property is fine":    {source: "{{.room.description}}", data: map[string]any{"room": Entity{"id": "r"}}},
		"if makes it optional":        {source: "{{if .target}}to {{entity .target}}{{end}}", data: map[string]any{}},
		"with makes it optional":      {source: "{{with .target}}{{.name}}{{end}}", data: map[string]any{}},
		"range over missing is fine":  {source: "{{range .exits}}{{.}}{{end}}", data: map[string]any{}},
		"else of with is unprotected": {source: "{{with .a}}{{.}}{{else}}{{.b}}{{end}}", data: map[string]any{}, want: ".b isn't in the data"},
		"inside with, dot isn't data": {source: "{{with .room}}{{.name}}{{end}}", data: map[string]any{"room": Entity{}}},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ts := NewTemplates()
			add(t, ts, "v", FormatText, "game", tc.source)

			_, _, err := ts.Render("v", FormatText, "", tc.data)
			switch {
			case tc.want == "" && err != nil:
				t.Errorf("error = %v, want none", err)
			case tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)):
				t.Errorf("error = %v, want it to contain %q", err, tc.want)
			}
		})
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

func TestCommands(t *testing.T) {
	ts := NewTemplates()
	add(t, ts, "exits", FormatText, "game", `Exits: [C]{{command "go north" "north"}}[x], {{command "up"}}`)
	add(t, ts, "menu", FormatHTML, "game", `<li>{{command "buy \"rope\"" "Rope <cheap>"}}</li>`)
	add(t, ts, "menu", FormatText, "game", `{{command "buy rope" "Rope"}}`)

	if got := render(t, ts, "exits", FormatText, "", map[string]any{}); got != "Exits: [C]north[x], up" {
		t.Errorf("text: got %q", got)
	}

	got := render(t, ts, "exits", FormatHTML, "", map[string]any{})
	want := `Exits: <span class="ansi-fg-6 ansi-bold"><dragon-command value="go north">north</dragon-command></span>, <dragon-command value="up">up</dragon-command>`
	if got != want {
		t.Errorf("text as HTML:\n got %s\nwant %s", got, want)
	}

	got = render(t, ts, "menu", FormatHTML, "", map[string]any{})
	want = `<li><dragon-command value="buy &#34;rope&#34;">Rope &lt;cheap&gt;</dragon-command></li>`
	if got != want {
		t.Errorf("HTML:\n got %s\nwant %s", got, want)
	}
}

func TestCommandErrors(t *testing.T) {
	cases := map[string]string{
		`{{command ""}}`:           `command needs the command to run, like {{command "go north" "north"}}`,
		`{{command "go" "a" "b"}}`: `command takes the command and one label, like {{command "go north" "north"}}, but got 2 labels`,
	}
	for source, want := range cases {
		ts := NewTemplates()
		add(t, ts, "v", FormatText, "game", source)
		if _, _, err := ts.Render("v", FormatText, "", map[string]any{}); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: error = %v, want it to contain %q", source, err, want)
		}
	}
}

func TestOnlyBlocksNeedsABlock(t *testing.T) {
	ts := NewTemplates()
	add(t, ts, "say", FormatText, "dragon:chat", `{{define "actor"}}You say hi.{{end}}{{define "others"}}Someone says hi.{{end}}`)

	_, _, err := ts.Render("say", FormatText, "", map[string]any{})
	want := `dragon:chat/views/say.txt.tmpl rendered nothing: all of it is in blocks (actor, others). Send one as the third argument, like o:send("say", data, "actor").`
	if err == nil || err.Error() != want {
		t.Errorf("error = %v\nwant %s", err, want)
	}

	if got := render(t, ts, "say", FormatText, "others", map[string]any{}); got != "Someone says hi." {
		t.Errorf("with a block: %q", got)
	}
}
