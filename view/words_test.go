package view

import (
	"testing"
)

func TestArticles(t *testing.T) {
	data := map[string]any{
		"keeper": Entity{"id": "k", "name": "bartender"},
		"elf":    Entity{"id": "e", "name": "elf"},
		"water":  Entity{"id": "w", "name": "water", "article": "some"},
		"alice":  Entity{"id": "a", "name": "Alice", "proper": true},
		"sword":  Entity{"id": "s", "name": "sword", "article": ""},
	}

	tests := map[string]string{
		`{{The .keeper}} nods.`:     "The bartender nods.",
		`You hail {{the .keeper}}.`: "You hail the bartender.",
		`{{A .keeper}} arrives.`:    "A bartender arrives.",
		`You see {{a .elf}}.`:       "You see an elf.",
		`{{A .elf}} sings.`:         "An elf sings.",
		`You find {{a .water}}.`:    "You find some water.",
		`{{A .water}} pools.`:       "Some water pools.",
		`{{The .alice}} waves.`:     "Alice waves.",
		`You see {{a .alice}}.`:     "You see Alice.",
		`You see {{a .sword}}.`:     "You see sword.",
	}
	for source, want := range tests {
		ts := NewTemplates()
		add(t, ts, "v", FormatText, "game", source)
		if got := render(t, ts, "v", FormatText, "", data); got != want {
			t.Errorf("%s: got %q, want %q", source, got, want)
		}
	}

	// On the web the name stays clickable and the article is plain text.
	ts := NewTemplates()
	add(t, ts, "v", FormatText, "game", `{{The .keeper}} nods at {{a .elf}}.`)
	got := render(t, ts, "v", FormatHTML, "", data)
	want := `The <dragon-entity ref="k">bartender</dragon-entity> nods at an <dragon-entity ref="e">elf</dragon-entity>.`
	if got != want {
		t.Errorf("text as HTML:\n got %s\nwant %s", got, want)
	}

	add(t, ts, "v", FormatHTML, "game", `<p>{{A .keeper}} and {{the .alice}}</p>`)
	got = render(t, ts, "v", FormatHTML, "", data)
	want = `<p>A <dragon-entity ref="k">bartender</dragon-entity> and <dragon-entity ref="a">Alice</dragon-entity></p>`
	if got != want {
		t.Errorf("HTML:\n got %s\nwant %s", got, want)
	}
}

func TestCaseHelpers(t *testing.T) {
	data := map[string]any{
		"msg":    "hello there",
		"keeper": Entity{"id": "k", "name": "bartender"},
		"lowkey": Entity{"id": "l", "name": "the bartender"},
	}

	tests := map[string]string{
		`{{cap .msg}}`:              "Hello there",
		`[c]{{cap "[Y]hi"}}[x]`:     "[c][Y]Hi[x]",
		`{{upper "[c]shout[x]"}}`:   "[c]SHOUT[x]",
		`{{lower "LOUD [Y]NOISE"}}`: "loud [Y]noise",
		`{{cap (entity .lowkey)}}`:  "The bartender",
		`{{cap ""}}`:                "",
	}
	for source, want := range tests {
		ts := NewTemplates()
		add(t, ts, "v", FormatText, "game", source)
		if got := render(t, ts, "v", FormatText, "", data); got != want {
			t.Errorf("%s: got %q, want %q", source, got, want)
		}
	}

	// Markup and ids are left alone; only the name changes.
	ts := NewTemplates()
	add(t, ts, "v", FormatText, "game", `{{cap (entity .lowkey)}} and {{upper (entity .keeper)}}`)
	got := render(t, ts, "v", FormatHTML, "", data)
	want := `<dragon-entity ref="l">The bartender</dragon-entity> and <dragon-entity ref="k">BARTENDER</dragon-entity>`
	if got != want {
		t.Errorf("text as HTML:\n got %s\nwant %s", got, want)
	}

	add(t, ts, "v", FormatHTML, "game", `<b>{{cap (entity .lowkey)}}</b> {{upper .msg}} {{cap "<i>x</i>"}}`)
	got = render(t, ts, "v", FormatHTML, "", data)
	want = `<b><dragon-entity ref="l">The bartender</dragon-entity></b> HELLO THERE &lt;i&gt;X&lt;/i&gt;`
	if got != want {
		t.Errorf("HTML:\n got %s\nwant %s", got, want)
	}
}
