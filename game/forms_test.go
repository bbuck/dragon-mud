package game

import (
	"strings"
	"testing"
	"testing/fstest"
)

var shop = fstest.MapFS{
	"plugin.lua": file(`return { name = "game" }`),
	"hooks.lua": file(`
				local forms = require("dragon.forms")
		local shop = forms.new {
			{ "buy <count:number> <item>", function(actor, args, who)
				actor:send(who .. ": " .. args.count .. " " .. args.item .. ", coming up.")
			end },
			{ "list", function(actor, args, who) actor:send(who .. ": apples.") end },
		}

		return {
			["dragon:unmatched_input"] = function(event)
				local ok, miss = shop:parse(event.actor, event.line, "Shopkeeper")
				if not ok and miss.reason then
					event.actor:send("Shopkeeper: " .. miss.reason)
					ok = true
				end
				event.handled = ok
				return event
			end,
		}
	`),
	"commands.lua": file(`
				local world = require("dragon.world")
		return {
			whoami = { execute = function(actor)
				local npc = world.create({})
				actor:send("player " .. tostring(actor:is_player()) .. ", npc " .. tostring(npc:is_player()))
			end },
		}
	`),
}

func TestFormSetsHandleUnmatchedInput(t *testing.T) {
	g := startGame(t, shop)

	alice := connect(t, g)
	alice.login("Alice")

	alice.send("buy 3 apples")
	alice.expect("Shopkeeper: 3 apples, coming up.")
	alice.send("list")
	alice.expect("Shopkeeper: apples.")

	// The shape fit but a slot didn't resolve: the handler used the reason.
	alice.send("buy lots apples")
	alice.expect("Shopkeeper: 'lots' isn't a number.")

	// No form fit, so the handler left it unhandled.
	alice.send("dance")
	alice.expect("Huh?")
}

func TestIsPlayer(t *testing.T) {
	g := startGame(t, shop)

	alice := connect(t, g)
	alice.login("Alice")

	alice.send("whoami")
	alice.expect("player true, npc false")
}

func TestFormSetErrors(t *testing.T) {
	cases := map[string]struct {
		hooks string
		want  string
	}{
		"not a list": {
			hooks: `forms.new("buy <item>")`,
			want:  `game/hooks.lua:2: dragon.forms.new: takes a list of forms, like forms.new { { "buy <item>", function(actor, args) ... end } }, not a string.`,
		},
		"empty": {
			hooks: `forms.new {}`,
			want:  `dragon.forms.new: was given no forms. Pass at least one`,
		},
		"no function": {
			hooks: `forms.new { { "buy <item>" } }`,
			want:  `dragon.forms.new: form #1 ("buy <item>") needs a function after its pattern`,
		},
		"unknown slot type": {
			hooks: `forms.new { { "buy <item:thingy>", function() end } }`,
			want:  `uses type "thingy", which no loaded plugin provides.`,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := newGame(t, fstest.MapFS{
				"plugin.lua": file(`return { name = "game" }`),
				"hooks.lua":  file("local forms = require(\"dragon.forms\")\n" + tc.hooks + "\nreturn {}"),
			})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}
