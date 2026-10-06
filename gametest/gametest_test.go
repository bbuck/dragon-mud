package gametest

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	"bbuck.dev/dragon-mud/builtin"
	"bbuck.dev/dragon-mud/plugin"
	"bbuck.dev/dragon-mud/scripting"
	"bbuck.dev/dragon-mud/scripting/lua"
)

func runSuite(t *testing.T, tests fstest.MapFS, pattern string) (Result, string) {
	t.Helper()

	var sources []plugin.Source
	for _, name := range builtin.Names {
		files, err := builtin.FS(name)
		if err != nil {
			t.Fatal(err)
		}
		sources = append(sources, plugin.Source{Origin: name, Files: files, Builtin: true})
	}
	sources = append(sources, plugin.Source{Origin: "game", Game: true, Files: fstest.MapFS{
		"init.lua": {Data: []byte(`return { events = { handlers = { ["dragon:player_connected"] = function(event) event.actor:send("Hello, " .. event.actor:get("name") .. ".") end } } }`)},
	}})

	var out strings.Builder
	opts := Options{
		Name:      "Test",
		NewEngine: func() scripting.Engine { return lua.New() },
		Plugins:   sources,
		Suites:    []Suite{{Origin: "game/tests", Files: tests}},
		Out:       &out,
	}
	if pattern != "" {
		opts.Run = regexp.MustCompile(pattern)
	}
	result, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}

	return result, out.String()
}

func TestPassingAndFailingTests(t *testing.T) {
	result, out := runSuite(t, fstest.MapFS{
		"hello_test.lua": {Data: []byte(`
			local helpers = require("helpers")
			return {
				["players are greeted"] = function(t)
					local p = helpers.player(t, "Alice")
					p:expect("Hello, Alice.")
				end,
				["a missing line fails"] = function(t)
					local p = t:connect()
					p:expect("Goodbye", 0.1)
				end,
				["forbidden output fails"] = function(t)
					local p = t:connect()
					p:send("Bob")
					p:expect_without("Create a new account?", "By what name")
				end,
				["eval returns objects as ids"] = function(t)
					local id = t:eval([[ return require("dragon.world").create({ key = "hall" }) ]])
					assert(type(id) == "string" and #id == 8, "got " .. tostring(id))
				end,
			}
		`)},
		"helpers.lua": {Data: []byte(`
			return { player = function(t, name) local p = t:connect(); p:login(name); return p end }
		`)},
	}, "")

	if result.Passed != 2 || result.Failed != 2 {
		t.Errorf("result = %+v\n%s", result, out)
	}
	for _, want := range []string{
		"game/tests/hello_test.lua\n",
		"  ok    players are greeted",
		"  FAIL  a missing line fails",
		`never received "Goodbye" within 100ms; received "Welcome to Test!", "By what name shall we know you?"`,
		"  FAIL  forbidden output fails",
		`received "By what name" before "Create a new account?"`,
		"  ok    eval returns objects as ids",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output doesn't contain %q:\n%s", want, out)
		}
	}
}

func TestRunPicksTests(t *testing.T) {
	result, _ := runSuite(t, fstest.MapFS{
		"a_test.lua": {Data: []byte(`return { ["one"] = function() end, ["two"] = function() error("no") end }`)},
	}, "^one$")
	if result.Passed != 1 || result.Failed != 0 {
		t.Errorf("result = %+v", result)
	}
}

func TestBrokenFileFails(t *testing.T) {
	result, out := runSuite(t, fstest.MapFS{
		"broken_test.lua": {Data: []byte(`return 42`)},
	}, "")
	if result.Failed != 1 || !strings.Contains(out, "returns a number, but a test file returns a table of tests") {
		t.Errorf("result = %+v\n%s", result, out)
	}
}
