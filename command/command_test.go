package command

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"bbuck.dev/dragon-mud/scripting"
)

// fn is a script function stand-in that records its name.
type fn string

func (f fn) Call(context.Context, ...any) (any, error)      { return nil, nil }
func (f fn) CallAll(context.Context, ...any) ([]any, error) { return nil, nil }

var _ scripting.Function = fn("")

// people is a slot type that resolves names of people in a fixed room.
func people(here ...string) SlotType {
	return SlotType{
		Name: "person", Plugin: "test", Modifiers: []string{"here", "anywhere"},
		Resolve: func(_ context.Context, _ any, text string, mods Modifiers) (any, bool, string, error) {
			for _, name := range here {
				if strings.EqualFold(name, text) {
					return name, true, "", nil
				}
			}
			return nil, false, fmt.Sprintf("You don't see '%s' here.", text), nil
		},
	}
}

func registry(t *testing.T, slots []SlotType, defs ...CommandDef) *Registry {
	t.Helper()

	r := NewRegistry()
	for _, s := range slots {
		if err := r.AddSlot(s, false); err != nil {
			t.Fatal(err)
		}
	}
	for _, d := range defs {
		if d.Plugin == "" {
			d.Plugin = "test"
		}
		if err := r.Add(d); err != nil {
			t.Fatal(err)
		}
	}

	return r
}

func forms(patterns ...string) []FormDef {
	var defs []FormDef
	for _, p := range patterns {
		defs = append(defs, FormDef{Pattern: p, Execute: fn(p)})
	}
	return defs
}

func parse(t *testing.T, r *Registry, input string) (string, map[string]any, *NoMatch) {
	t.Helper()

	m, miss, err := r.Parse(context.Background(), nil, input)
	if err != nil {
		t.Fatal(err)
	}
	if m == nil {
		return "", nil, miss
	}
	return string(m.Form.Execute.(fn)), m.Args, nil
}

func TestTokenize(t *testing.T) {
	tests := map[string][]string{
		"say hello there":       {"say", "hello", "there"},
		`say "hi to bob"`:       {"say", "hi to bob"},
		`say "unclosed quote`:   {"say", "unclosed quote"},
		"'hello":                {"'", "hello"},
		"@dig  The   Cellar":    {"@", "dig", "The", "Cellar"},
		"  ":                    nil,
		`tell bob "a" b`:        {"tell", "bob", "a", "b"},
		"emote 'quotes' inside": {"emote", "'quotes'", "inside"},
	}

	for input, want := range tests {
		var got []string
		for _, tok := range Tokenize(input) {
			got = append(got, tok.Text)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Tokenize(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestParsePattern(t *testing.T) {
	patterns, err := ParsePattern("look [at] <thing:person:here,anywhere>")
	if err != nil {
		t.Fatal(err)
	}
	if len(patterns) != 2 {
		t.Fatalf("got %d patterns, want 2", len(patterns))
	}
	if got := patterns[1].key(); got != " look at <person:anywhere,here>" {
		t.Errorf("key = %q", got)
	}

	for _, bad := range []string{
		"", "say <Message>", "say <a> <a>", "say [unclosed", "say [a [b]]", "say <a:b:c:d>", "[]", "say <x:t:>",
	} {
		if _, err := ParsePattern(bad); err == nil {
			t.Errorf("ParsePattern(%q) succeeded", bad)
		}
	}
}

func TestMaximalMunch(t *testing.T) {
	r := registry(t, []SlotType{people("Bob", "the guard")},
		CommandDef{Name: "say", Forms: forms("say <message>", "say <message> to <target:person:here>", "'<message>")},
	)

	tests := []struct {
		input, form string
		args        map[string]any
	}{
		{"say hi", "say <message>", map[string]any{"message": "hi"}},
		{"say hi to bob", "say <message> to <target:person:here>", map[string]any{"message": "hi", "target": "Bob"}},
		{"say hi to the store", "say <message>", map[string]any{"message": "hi to the store"}},
		{"say meet me to the left to bob", "say <message> to <target:person:here>",
			map[string]any{"message": "meet me to the left", "target": "Bob"}},
		{"say go to the guard", "say <message> to <target:person:here>", map[string]any{"message": "go", "target": "the guard"}},
		{`say "hi to bob"`, "say <message>", map[string]any{"message": "hi to bob"}},
		{"say  Spacing   KEPT", "say <message>", map[string]any{"message": "Spacing   KEPT"}},
		{"'hello there", "'<message>", map[string]any{"message": "hello there"}},
		{"SAY hi", "say <message>", map[string]any{"message": "hi"}},
	}

	for _, tt := range tests {
		form, args, miss := parse(t, r, tt.input)
		if miss != nil {
			t.Errorf("%q: no match (%+v)", tt.input, miss)
			continue
		}
		if form != tt.form || !reflect.DeepEqual(args, tt.args) {
			t.Errorf("%q: got %q %v, want %q %v", tt.input, form, args, tt.form, tt.args)
		}
	}
}

func TestNoMatch(t *testing.T) {
	r := registry(t, []SlotType{people("Bob")},
		CommandDef{Name: "give", Forms: forms("give <thing> to <target:person:here>")},
		CommandDef{Name: "say", Forms: forms("say <message>")},
	)

	_, _, miss := parse(t, r, "give sword to alice")
	if miss == nil || miss.Reason != "You don't see 'alice' here." {
		t.Errorf("unresolved target: %+v", miss)
	}

	_, _, miss = parse(t, r, "say")
	if miss == nil || !reflect.DeepEqual(miss.Usage, []string{"say <message>"}) {
		t.Errorf("missing message: %+v", miss)
	}

	_, _, miss = parse(t, r, "fly")
	if miss == nil || miss.Reason != "" || miss.Usage != nil {
		t.Errorf("unknown command: %+v", miss)
	}
}

func TestSpecificity(t *testing.T) {
	r := registry(t, []SlotType{people("Bob", "at bob")},
		CommandDef{Name: "look", Forms: forms("look", "look <thing:person:here>", "look at <thing:person:here>")},
		CommandDef{Name: "count", Forms: forms("count <n:number>", "count <what>")},
	)

	tests := map[string]string{
		"look":        "look",
		"look bob":    "look <thing:person:here>",
		"look at bob": "look at <thing:person:here>",
		"count 3":     "count <n:number>",
		"count sheep": "count <what>",
	}
	for input, want := range tests {
		if form, _, _ := parse(t, r, input); form != want {
			t.Errorf("%q matched %q, want %q", input, form, want)
		}
	}
}

func TestLaterPluginsWinTies(t *testing.T) {
	// Each form has one literal and one text slot, so only precedence
	// separates them.
	r := registry(t, nil,
		CommandDef{Name: "dance", Plugin: "dragon:chat", Forms: forms("dance <style>")},
		CommandDef{Name: "waltz", Plugin: "game", Forms: forms("<step> slowly")},
	)

	if form, _, _ := parse(t, r, "dance slowly"); form != "<step> slowly" {
		t.Errorf("tie went to %q, want the game's form", form)
	}
}

func TestAdditiveAndReplace(t *testing.T) {
	r := registry(t, []SlotType{people("Bob")},
		CommandDef{Name: "say", Plugin: "dragon:chat", Desc: "Say something.", Forms: forms("say <message>")},
		CommandDef{Name: "say", Plugin: "game", Forms: forms("say <message> to <target:person:here>")},
	)

	cmd, _ := r.Lookup("say")
	if len(cmd.Forms) != 2 || cmd.Desc != "Say something." {
		t.Fatalf("say has %d forms, desc %q", len(cmd.Forms), cmd.Desc)
	}

	err := r.Add(CommandDef{Name: "say", Plugin: "game", Replace: true, Desc: "Talk.", Forms: forms("say <words>")})
	if err != nil {
		t.Fatal(err)
	}
	cmd, _ = r.Lookup("say")
	if len(cmd.Forms) != 1 || cmd.Desc != "Talk." || cmd.Plugin != "game" {
		t.Errorf("after replace: %d forms, desc %q, plugin %q", len(cmd.Forms), cmd.Desc, cmd.Plugin)
	}

	if err := r.Add(CommandDef{Name: "say", Plugin: "game", Replace: true}); err != nil {
		t.Fatal(err)
	}
	if _, ok := r.Lookup("say"); ok {
		t.Error("replacing with no forms didn't remove the command")
	}
}

func TestDuplicateFormErrors(t *testing.T) {
	base := func() *Registry {
		return registry(t, nil,
			CommandDef{Name: "say", Plugin: "dragon:chat", Forms: forms("say <message>")},
		)
	}

	tests := []struct {
		def  CommandDef
		want string
	}{
		{
			CommandDef{Name: "say", Plugin: "game", Forms: forms("say <words>")},
			`game: the "say" form "say <words>" matches exactly the same input as "say <message>", already defined by dragon:chat. Remove it from game, or set replace = true on "say" in game`,
		},
		{
			CommandDef{Name: "talk", Plugin: "game", Forms: forms("SAY <words>")},
			`from dragon:chat's "say" command. Players couldn't reach one of them; remove it from game`,
		},
		{
			CommandDef{Name: "chat", Plugin: "game", Forms: forms("chat <a>", "chat <b>")},
			`also in game's "chat" command. Remove one of them.`,
		},
	}

	for _, tt := range tests {
		err := base().Add(tt.def)
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("Add(%s) error = %v\nwant it to contain %q", tt.def.Name, err, tt.want)
		}
	}

	// replace = true resolves the same-command case.
	r := base()
	if err := r.Add(CommandDef{Name: "say", Plugin: "game", Replace: true, Forms: forms("say <words>")}); err != nil {
		t.Errorf("replace didn't allow redefining the form: %v", err)
	}
}

func TestSlotChecks(t *testing.T) {
	r := registry(t, []SlotType{people()})

	tests := map[string]string{
		"get <thing:item>":         `game: the "get" form "get <thing:item>": slot <thing> uses type "item", which no loaded plugin provides. Known types: number, person, text, word.`,
		"get <who:persn>":          `type "persn", which no loaded plugin provides. Did you mean "person"?`,
		"get <who:person:hree>":    `slot <who> gives type "person" the modifier "hree", which it doesn't have. Did you mean "here"? "person"'s modifiers: here, anywhere.`,
		"get <n:number:big>":       `"number" takes no modifiers. Remove ":big".`,
		"get <who:person:here,,x>": `slot <who> has an empty modifier in "here,,x". Remove the extra comma, | or colon.`,
	}
	for pattern, want := range tests {
		err := r.Add(CommandDef{Name: "get", Plugin: "game", Forms: forms(pattern)})
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: error = %v, want %q", pattern, err, want)
		}
	}

	if err := r.AddSlot(people(), false); err == nil || !strings.Contains(err.Error(),
		`test: slot type "person" is already provided by test. Rename yours, or set replace = true on it`) {
		t.Errorf("duplicate slot type error = %v", err)
	}
	if err := r.AddSlot(people(), true); err != nil {
		t.Errorf("replacing a slot type: %v", err)
	}
}

func TestResolversAreCached(t *testing.T) {
	calls := 0
	counting := SlotType{
		Name: "person", Plugin: "test",
		Resolve: func(_ context.Context, _ any, text string, _ Modifiers) (any, bool, string, error) {
			calls++
			return text, text == "bob", "nope", nil
		},
	}
	r := registry(t, []SlotType{counting},
		CommandDef{Name: "say", Forms: forms("say <message> to <target:person>", "tell <target:person> <message>")},
	)

	// "to bob" can end the message at either "to"; bob resolves once.
	parse(t, r, "say a to b to bob")
	if calls != 2 { // "b to bob" fails, then "bob"
		t.Errorf("resolver called %d times, want 2", calls)
	}
}

func TestDidYouMean(t *testing.T) {
	options := []string{"here", "held", "anywhere", "online"}
	tests := map[string]string{
		"hree":    "here",
		"hold":    "held",
		"anywher": "anywhere",
		"onlien":  "online",
		"xyz":     "",
		"h":       "",
	}
	for typo, want := range tests {
		got := DidYouMean(typo, options)
		if want == "" && got != "" || want != "" && !strings.Contains(got, `"`+want+`"`) {
			t.Errorf("DidYouMean(%q) = %q, want %q", typo, got, want)
		}
	}
}

func TestAbbreviations(t *testing.T) {
	r := registry(t, []SlotType{people("Bob")},
		CommandDef{Name: "down", Forms: forms("d(own)")},
		CommandDef{Name: "look", Forms: forms("l(ook) [at] <thing:person:here>", "l(ook)")},
		CommandDef{Name: "go", Forms: forms("go <where>")},
		CommandDef{Name: "goto", Forms: forms("g(oto) <where>")},
	)

	tests := map[string]string{
		"d":          "d(own)",
		"dow":        "d(own)",
		"DOWN":       "d(own)",
		"l":          "l(ook)",
		"lo bob":     "l(ook) [at] <thing:person:here>",
		"l at bob":   "l(ook) [at] <thing:person:here>",
		"go north":   "go <where>", // typed in full beats abbreviated
		"got north":  "g(oto) <where>",
		"goto north": "g(oto) <where>",
	}
	for input, want := range tests {
		if form, _, _ := parse(t, r, input); form != want {
			t.Errorf("%q matched %q, want %q", input, form, want)
		}
	}

	for _, input := range []string{"downs", "dx", "looker"} {
		if form, _, _ := parse(t, r, input); form != "" {
			t.Errorf("%q matched %q, want nothing", input, form)
		}
	}

	// Usage counts abbreviated first words too.
	if _, _, miss := parse(t, r, "g"); miss == nil || !reflect.DeepEqual(miss.Usage, []string{"g(oto) <where>"}) {
		t.Errorf("usage for g: %+v", miss)
	}
}

func TestAbbreviationErrors(t *testing.T) {
	want := "has parentheses that don't make an abbreviation. Write the shortest form players may type, then the rest of the word in parentheses, like d(own) for d, do, dow and down."
	for _, bad := range []string{"d(own", "down)", "(down)", "d()", "d(o)(wn)", "d(o)wn"} {
		_, err := ParsePattern(bad)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("ParsePattern(%q) error = %v", bad, err)
		}
	}
}

func TestModifierSyntax(t *testing.T) {
	patterns, err := ParsePattern("look <thing:person:here|anywhere,here>")
	if err != nil {
		t.Fatal(err)
	}
	mods := patterns[0].Elements[1].Slot.Modifiers
	if want := (Modifiers{{"here", "anywhere"}, {"here"}}); !reflect.DeepEqual(mods, want) {
		t.Errorf("modifiers = %v, want %v", mods, want)
	}
	if mods.String() != "here|anywhere,here" {
		t.Errorf("String() = %q", mods.String())
	}

	// The same modifiers in another order are the same form.
	r := registry(t, []SlotType{people("Bob")}, CommandDef{Name: "look", Forms: forms("look <thing:person:here|anywhere>")})
	err = r.Add(CommandDef{Name: "look", Plugin: "test", Forms: forms("look <thing:person:anywhere|here>")})
	if err == nil {
		t.Error("here|anywhere and anywhere|here should be the same form")
	}

	for _, bad := range []string{"look <t:person:here|>", "look <t:person:|here>", "look <t:person:here,>"} {
		if _, err := ParsePattern(bad); err == nil || !strings.Contains(err.Error(), "has an empty modifier") {
			t.Errorf("ParsePattern(%q) error = %v", bad, err)
		}
	}
}
