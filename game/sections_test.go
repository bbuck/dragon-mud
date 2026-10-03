package game

import (
	"maps"
	"strings"
	"testing"
	"testing/fstest"
)

// sectioned is a game whose room kind has an exits section that a plugin
// and the game both add to.
func sectioned(extra fstest.MapFS) fstest.MapFS {
	files := fstest.MapFS{
		"plugin.lua": file(`return { name = "game" }`),
		"commands.lua": file(`
			return { look = { replace = true, forms = { { "look", function(actor)
				actor:send("room", { title = "The Dragon's Rest", here = actor })
			end } } } }
		`),
		"views/room.txt.tmpl":    file("[Y]{{.title}}[x]\n{{section \"exits\"}}"),
		"views/room.html.tmpl":   file(`<h2>{{.title}}</h2>{{section "exits"}}`),
		"views/minimap.txt.tmpl": file(`[map of {{.place}} for {{entity .viewer}}]`),
		"hooks.lua": file(`
			return {
				["section:room.exits"] = {
					before = { "dragon:chat" },
					handler = function(event)
						table.insert(event.parts, { view = "minimap", data = { place = event.data.title, viewer = event.data.here } })
						return event
					end,
				},
			}
		`),
	}
	maps.Copy(files, extra)

	return files
}

func TestSectionsCollectParts(t *testing.T) {
	g := startGame(t, sectioned(nil))

	alice := connect(t, g)
	alice.login("Alice")
	alice.send("look")
	m := alice.expect("[Y]The Dragon's Rest[x]\n[map of The Dragon's Rest for Alice]")
	if want := `<h2>The Dragon&#39;s Rest</h2>[map of The Dragon&#39;s Rest for <dragon-entity ref="`; !strings.HasPrefix(m.HTML, want) {
		t.Errorf("HTML = %s, want it to start with %s", m.HTML, want)
	}
}

func TestSectionsAreWired(t *testing.T) {
	// The game's own handler always comes from game/hooks.lua, so a second
	// plugin's part is shown by the wiring disabling the game's.
	g := startGame(t, sectioned(fstest.MapFS{
		"wiring.lua": file(`return { hooks = { ["section:room.exits"] = { disable = { "game" } } } }`),
	}))

	alice := connect(t, g)
	alice.login("Alice")
	alice.send("look")
	m := alice.expect("[Y]The Dragon's Rest[x]")
	if strings.Contains(m.Text, "map of") {
		t.Errorf("disabled part still shown: %q", m.Text)
	}
}

func TestSectionErrors(t *testing.T) {
	tests := []struct {
		name  string
		files fstest.MapFS
		want  string
	}{
		{
			"unknown kind",
			fstest.MapFS{"hooks.lua": file(`return { ["section:rom.exits"] = function(event) end }`)},
			`game/hooks.lua: section:rom.exits adds to the view "rom", which no plugin defines. Did you mean "room"?`,
		},
		{
			"unknown section",
			fstest.MapFS{"hooks.lua": file(`return { ["section:room.exist"] = function(event) end }`)},
			`game/hooks.lua: section:room.exist adds to the "exist" section of "room", but the room template has no {{section "exist"}}. Did you mean "exits"? Its sections: exits.`,
		},
		{
			"HTML is missing a section",
			fstest.MapFS{"views/room.html.tmpl": file(`<h2>{{.title}}</h2>`)},
			`game/views/room.html.tmpl has no {{section "exits"}}, but game/views/room.txt.tmpl does.`,
		},
		{
			"bad hook name",
			fstest.MapFS{"hooks.lua": file(`return { ["section:room"] = function(event) end }`)},
			`or section:<kind>.<section> to add to a message's section, like section:room.exits.`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newGame(t, sectioned(tt.files))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %v, want it to contain %q", err, tt.want)
			}
		})
	}
}

func TestBadSectionPart(t *testing.T) {
	g := startGame(t, sectioned(fstest.MapFS{
		"hooks.lua": file(`
			return { ["section:room.exits"] = function(event)
				table.insert(event.parts, { view = "minimpa" })
				return event
			end }
		`),
	}))

	alice := connect(t, g)
	alice.login("Alice")
	alice.send("look")
	alice.expect(`section:room.exits: part 1: there's no view "minimpa". Did you mean "minimap"?`)
}
