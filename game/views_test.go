package game

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"bbuck.dev/dragon-mud/message"
	"bbuck.dev/dragon-mud/scaffold"
	"bbuck.dev/dragon-mud/session"
)

// expectReply waits for the reply to the request id.
func (c *client) expectReply(id string) message.Message {
	c.t.Helper()

	timeout := time.After(2 * time.Second)
	for {
		select {
		case m := <-c.conn.messages:
			if m.Reply == id {
				return m
			}
		case <-timeout:
			c.t.Fatalf("never received a reply to request %s", id)
		}
	}
}

// expectNone fails if a message containing unwanted arrives soon.
func (c *client) expectNone(unwanted string) {
	c.t.Helper()

	timeout := time.After(100 * time.Millisecond)
	for {
		select {
		case m := <-c.conn.messages:
			if strings.Contains(m.Text, unwanted) {
				c.t.Fatalf("received %q", m.Text)
			}
		case <-timeout:
			return
		}
	}
}

func file(source string) *fstest.MapFile {
	return &fstest.MapFile{Data: []byte(source)}
}

var fighting = fstest.MapFS{
	"plugin.lua": file(`return { name = "game" }`),
	"commands.lua": file(`
		return {
			hit = { forms = { { "hit <target:object:online>", function(actor, args)
				local data = { actor = actor, target = args.target }
				actor:send("hit", data, "actor")
				args.target:send("hit", data, "target")
				game.broadcast("hit", data, "others", { actor, args.target })
			end } } },
			shout = { forms = { { "shout <text>", function(actor, args)
				game.broadcast("shout", { actor = actor, text = args.text }, nil, actor)
			end } } },
			show = { execute = function(actor)
				actor:set_parent(world.create{ properties = { title = "the Bold" } })
				actor:set("weapon", world.create{ properties = { name = "sword", damage = 5 } })
				actor:send("show", { actor = actor, items = { "a", "b" } })
			end },
			oops = { forms = {
				{ "oops kind", function(actor) actor:send("hti", {}) end },
				{ "oops block", function(actor) actor:send("hit", { actor = actor }, "atcor") end },
				{ "oops data", function(actor) actor:send("hit", { actor = actor, f = function() end }) end },
			} },
		}
	`),
	"views/hit.txt.tmpl": file(`
{{define "actor"}}You hit {{entity .target}}.{{end}}
{{define "target"}}{{entity .actor}} hits you.{{end}}
{{define "others"}}{{entity .actor}} hits {{entity .target}}.{{end}}
`),
	"views/shout.txt.tmpl":  file("[Y]{{entity .actor}} shouts, \"{{.text}}\"[x]\n"),
	"views/shout.html.tmpl": file(`<p class="shout">{{entity .actor}}: {{.text}}</p>`),
	"views/show.txt.tmpl": file(
		`{{.actor.name}} {{.actor.title}} wields {{.actor.weapon.name}}` +
			` ({{if .actor.weapon.damage}}full{{else}}name only{{end}}) {{range .items}}{{.}}{{end}}`),
}

func TestMessageKinds(t *testing.T) {
	g := startGame(t, fighting)

	alice := connect(t, g)
	alice.login("Alice")
	bob := connect(t, g)
	bob.login("Bob")
	carol := connect(t, g)
	carol.login("Carol")

	alice.send("hit bob")
	alice.expect("You hit Bob.")
	bob.expect("Alice hits you.")
	m := carol.expect("Alice hits Bob.")

	// The web gets the text template as HTML, with entities to click.
	if m.Kind != "hit" || !strings.Contains(m.HTML, `<dragon-entity ref="`) || !strings.Contains(m.HTML, `">Bob</dragon-entity>.`) {
		t.Errorf("hit message = %+v", m)
	}

	alice.send("shout hello <there>")
	m = bob.expect(`Alice shouts, "hello <there>"`)
	if !strings.Contains(m.HTML, `<p class="shout"><dragon-entity ref="`) || !strings.Contains(m.HTML, `: hello &lt;there&gt;</p>`) {
		t.Errorf("shout HTML = %s", m.HTML)
	}
	alice.expectNone("shouts")
}

func TestMessageEntities(t *testing.T) {
	g := startGame(t, fighting)

	alice := connect(t, g)
	alice.login("Alice")

	alice.send("show")
	alice.expect("Alice the Bold wields sword (name only) ab")
}

func TestMessageErrors(t *testing.T) {
	g := startGame(t, fighting)

	alice := connect(t, g)
	alice.login("Alice")

	alice.send("oops kind")
	alice.expect(`there's no view "hti". Add views/hti.txt.tmpl to your plugin to define it. Did you mean "hit"?`)

	alice.send("oops block")
	alice.expect(`game/views/hit.txt.tmpl has no block "atcor". It defines actor, others, target; add {{define "atcor"}}...{{end}} to it, or send one of those.`)

	alice.send("oops data")
	alice.expect(`argument #2: f: a function can't be shown in a message`)
}

func TestMessageTemplatesReload(t *testing.T) {
	files := fstest.MapFS{}
	for name, f := range fighting {
		files[name] = f
	}
	g := startGame(t, files)

	alice := connect(t, g)
	alice.login("Alice")
	bob := connect(t, g)
	bob.login("Bob")

	files["views/hit.txt.tmpl"] = file(`{{define "actor"}}You wallop {{entity .target}}!{{end}}{{define "target"}}{{end}}{{define "others"}}{{end}}`)
	g.Reload()

	alice.send("hit bob")
	alice.expect("You wallop Bob!")
}

func TestMessageFileErrors(t *testing.T) {
	tests := []struct {
		name  string
		files fstest.MapFS
		want  string
	}{
		{
			"HTML without text",
			fstest.MapFS{"views/hit.html.tmpl": file("<b>hit</b>")},
			"game/views/hit.html.tmpl has no text version. Add views/hit.txt.tmpl: telnet shows it, and the web does whenever there's no HTML version.",
		},
		{
			"misnamed file",
			fstest.MapFS{"views/hit.tmpl": file("hit")},
			"game/views/hit.tmpl isn't a template name. Name templates <name>.txt.tmpl or <name>.html.tmpl, where the name is lowercase letters, digits and underscores, like say.txt.tmpl.",
		},
		{
			"directory",
			fstest.MapFS{"views/combat/hit.txt.tmpl": file("hit")},
			"game/views/combat is a directory, but views/ only holds template files like say.txt.tmpl.",
		},
		{
			"parse error",
			fstest.MapFS{"templates/entity_tooltip.html.tmpl": file("{{.entity")},
			"game/templates/entity_tooltip.html.tmpl:1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.files["plugin.lua"] = file(`return { name = "game" }`)
			_, err := newGame(t, tt.files)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %v\nwant it to contain %q", err, tt.want)
			}
		})
	}

	// Dotfiles are ignored.
	_, err := newGame(t, fstest.MapFS{
		"plugin.lua":      file(`return { name = "game" }`),
		"views/.DS_Store": file(""),
	})
	if err != nil {
		t.Errorf("a dotfile in views/ broke loading: %v", err)
	}
}

var clickable = fstest.MapFS{
	"plugin.lua": file(`return { name = "game" }`),
	"commands.lua": file(`
		return {
			make = { forms = { { "make <what>", function(actor, args)
				local here = world.create{ location = actor.location, properties = { name = args.what } }
				local held = world.create{ location = actor, properties = { name = args.what } }
				local away = world.create{ properties = { name = args.what } }
				actor:send("ids " .. here.id .. " " .. held.id .. " " .. away.id)
			end } } },
			go = { execute = function(actor) actor:move_to(world.create()) actor:send("Moved.") end },
			poke = { forms = {
				{ "poke <thing:object>", function(actor, args) actor:send("You poke " .. args.thing:get("name") .. " " .. args.thing.id .. ".") end },
				{ "pocket <thing:object:held>", function(actor, args) actor:send("Pocketed " .. args.thing.id .. ".") end },
				{ "summon <thing:object:anywhere>", function(actor, args) actor:send("Summoned " .. args.thing.id .. ".") end },
			} },
		}
	`),
	"hooks.lua": file(`
		return {
			get_tooltip = function(event)
				if event.entity:get("name") == "secret" then return false end
				if event.entity:get("name") == "rock" then
					event.block = "rock"
					event.weight = 3
					return event
				end
			end,
			get_default_action = function(event)
				if event.entity:get("name") == "rock" then
					event.command = "poke #" .. event.entity.id
					return event
				end
			end,
		}
	`),
	"templates/entity_tooltip.html.tmpl": file(`
<b>{{.entity.name}}</b> seen by {{.viewer.name}}
{{define "rock"}}<b>A rock</b> weighing {{.weight}}{{end}}
`),
}

// ids runs make and returns the ids of the objects it made: one here, one
// held and one somewhere else.
func (c *client) ids(what string) (here, held, away string) {
	c.t.Helper()

	c.send("make " + what)
	fields := strings.Fields(c.expect("ids ").Text)

	return fields[1], fields[2], fields[3]
}

func TestObjectSlotByID(t *testing.T) {
	g := startGame(t, clickable)

	alice := connect(t, g)
	alice.login("Alice")
	alice.send("go")
	alice.expect("Moved.")

	here, held, away := alice.ids("rock")

	// By name it's ambiguous; by id it isn't.
	alice.send("poke rock")
	alice.expect("Which 'rock' do you mean?")
	alice.send("poke #" + here)
	alice.expect("You poke rock " + here + ".")
	alice.send("poke #" + held)
	alice.expect("You poke rock " + held + ".")

	// An id doesn't reach past the slot's modifiers.
	alice.send("poke #" + away)
	alice.expect("You don't see that here.")
	alice.send("pocket #" + here)
	alice.expect("You aren't carrying that.")
	alice.send("summon #" + away)
	alice.expect("Summoned " + away + ".")
}

func TestEntityTooltips(t *testing.T) {
	g := startGame(t, clickable)

	alice := connect(t, g)
	alice.login("Alice")
	alice.send("go")
	alice.expect("Moved.")

	tooltip := func(id, ref string) string {
		t.Helper()
		g.Request(alice.s, session.Request{ID: id, Name: "entity_tooltip", Data: map[string]any{"ref": ref}})
		return alice.expectReply(id).HTML
	}

	rock, _, _ := alice.ids("rock")
	if got := tooltip("1", rock); got != "<b>A rock</b> weighing 3" {
		t.Errorf("rock tooltip = %q", got)
	}

	apple, _, _ := alice.ids("apple")
	if got := tooltip("2", apple); got != "<b>apple</b> seen by Alice" {
		t.Errorf("apple tooltip = %q", got)
	}

	secret, _, _ := alice.ids("secret")
	if got := tooltip("3", secret); got != "" {
		t.Errorf("a cancelled tooltip rendered %q", got)
	}

	if got := tooltip("4", "nosuchid"); got != "" {
		t.Errorf("a missing object's tooltip rendered %q", got)
	}
}

func TestNoTooltipTemplate(t *testing.T) {
	g := startGame(t, fighting)

	alice := connect(t, g)
	alice.login("Alice")
	alice.send("show")
	alice.expect("wields")

	g.Request(alice.s, session.Request{ID: "1", Name: "entity_tooltip", Data: map[string]any{"ref": "anything"}})
	if got := alice.expectReply("1").HTML; got != "" {
		t.Errorf("tooltip without a template = %q", got)
	}
}

func TestEntityDefaultAction(t *testing.T) {
	g := startGame(t, clickable)

	alice := connect(t, g)
	alice.login("Alice")
	alice.send("go")
	alice.expect("Moved.")

	rock, _, _ := alice.ids("rock")
	g.Request(alice.s, session.Request{ID: "1", Name: "entity_action", Data: map[string]any{"ref": rock}})
	alice.expectReply("1")
	echo := alice.expect("poke #" + rock)
	if echo.Kind != message.KindEcho {
		t.Errorf("echo kind = %q", echo.Kind)
	}
	alice.expect("You poke rock " + rock + ".")

	// No command set: clicking does nothing.
	apple, _, _ := alice.ids("apple")
	g.Request(alice.s, session.Request{ID: "2", Name: "entity_action", Data: map[string]any{"ref": apple}})
	alice.expectReply("2")
	alice.expectNone("poke")
}

func TestRequestsBeforeLogin(t *testing.T) {
	g := startGame(t, clickable)

	alice := connect(t, g)
	alice.expect("By what name")

	g.Request(alice.s, session.Request{ID: "1", Name: "entity_tooltip", Data: map[string]any{"ref": "x"}})
	if got := alice.expectReply("1").HTML; got != "" {
		t.Errorf("tooltip before login = %q", got)
	}
}

// The game dragon new writes uses views, tooltips and default
// actions; this keeps it working.
func TestScaffoldedGame(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "mygame")
	if err := scaffold.New(dir, scaffold.Data{Name: "Test"}); err != nil {
		t.Fatal(err)
	}
	files := os.DirFS(filepath.Join(dir, "game"))
	copied := fstest.MapFS{}
	if err := fs.WalkDir(files, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(files, name)
		copied[name] = &fstest.MapFile{Data: data}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	g := startGame(t, copied)

	alice := connect(t, g)
	alice.login("Alice")
	if m := alice.expect("The barkeep looks up"); m.Kind != "ambient" {
		t.Errorf("greeting kind = %q, want ambient", m.Kind)
	}
	room := alice.expect("The Dragon's Rest")
	if room.Kind != "room" || !strings.Contains(room.HTML, `<h2 class="room-title">The Dragon&#39;s Rest</h2>`) {
		t.Errorf("look = %q: %s", room.Kind, room.HTML)
	}

	bob := connect(t, g)
	bob.login("Bob")

	bob.send("say hi")
	said := alice.expect(`Bob says, "hi"`)
	if said.Kind != "say" || !strings.Contains(said.HTML, `">Bob</dragon-entity> says`) {
		t.Errorf("say = %q: %s", said.Kind, said.HTML)
	}

	alice.send("dance")
	alice.expect("You dance a little jig.")
	m := bob.expect("Alice dances a little jig.")
	if !strings.Contains(m.HTML, `">Alice</dragon-entity> dances`) {
		t.Errorf("dance HTML = %s", m.HTML)
	}

	alice.send("dance with bob")
	alice.expect("You whirl Bob around the room.")
	bob.expect("Alice whirls you around the room.")

	alice.send("look bob")
	alice.expect("You see nothing special.")

	bob.send("describe")
	bob.expect("A line with only [c].[x] saves it")
	bob.send("Tall, with a crooked smile.")
	bob.send(".")
	bob.expect("Saved.")
	alice.send("look bob")
	alice.expect("Tall, with a crooked smile.")

	// Clicking Bob looks at him.
	_, id, _ := strings.Cut(m.HTML, `ref="`)
	id, _, _ = strings.Cut(id, `"`)
	g.Request(alice.s, session.Request{ID: "1", Name: "entity_action", Data: map[string]any{"ref": id}})
	alice.expect("look #" + id)
	alice.expect("You see nothing special.")

	g.Request(alice.s, session.Request{ID: "2", Name: "entity_tooltip", Data: map[string]any{"ref": id}})
	if got := alice.expectReply("2").HTML; got != "<strong>Alice</strong>" {
		t.Errorf("tooltip = %q", got)
	}
}
