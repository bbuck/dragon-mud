package view

import (
	"fmt"
	htmltemplate "html/template"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"bbuck.dev/dragon-mud/ansi"
)

// Article and case helpers. Names are stored bare ("bartender"), and
// templates add the article a sentence needs:
//
//	{{the .x}}   the bartender       {{The .x}}   The bartender
//	{{a .x}}     a bartender         {{A .x}}     A bartender
//
// An object with proper = true never gets an article ("Alice"). Its
// article property, if set, replaces "a" or "an" ("some water", or ""
// for none); otherwise a name starting with a vowel gets "an". The name
// is the entity, clickable like {{entity}}.
//
//	{{cap x}}    first letter upper case: {{cap .message}}
//	{{upper x}}  every letter upper case
//	{{lower x}}  every letter lower case
//
// All three leave color codes, entity markup and HTML tags alone, so
// {{cap (entity .x)}} capitalizes the name, not the markup.

// article is the article e takes: definite for "the", or "a"/"an"/its
// article property; "" for a proper name or an empty article.
func article(e Entity, definite bool) string {
	if proper, _ := e["proper"].(bool); proper {
		return ""
	}
	if definite {
		return "the"
	}
	if a, ok := e["article"].(string); ok {
		return a
	}

	first, _ := utf8.DecodeRuneInString(e.Name())
	if strings.ContainsRune("aeiouAEIOU", first) {
		return "an"
	}

	return "a"
}

// withArticle writes e's article and then the name written by name, which
// formats it as entity markup for the template's format. capital makes the
// first letter upper case.
func withArticle[T ~string](x any, definite, capital bool, name func(any) (T, error)) (T, error) {
	e, err := asEntity(x)
	if err != nil {
		return "", err
	}
	written, err := name(e)
	if err != nil {
		return "", err
	}

	out := written
	if a := article(e, definite); a != "" {
		out = T(a) + " " + written
	}
	if capital {
		out = T(capitalize(string(out)))
	}

	return out, nil
}

// articleFuncs are the article helpers for a template whose entities are
// written by name.
func articleFuncs[T ~string](name func(any) (T, error)) map[string]any {
	return map[string]any{
		"the": func(x any) (T, error) { return withArticle(x, true, false, name) },
		"The": func(x any) (T, error) { return withArticle(x, true, true, name) },
		"a":   func(x any) (T, error) { return withArticle(x, false, false, name) },
		"A":   func(x any) (T, error) { return withArticle(x, false, true, name) },
	}
}

// caseFuncs are cap, upper and lower. HTML templates keep HTML values HTML,
// so markup made by {{entity}} survives.
func caseFuncs(html bool) map[string]any {
	wrap := func(change func(string) string) any {
		if !html {
			return func(x any) (string, error) {
				s, err := asText(x)
				return change(s), err
			}
		}
		return func(x any) (any, error) {
			if h, ok := x.(htmltemplate.HTML); ok {
				return htmltemplate.HTML(change(string(h))), nil
			}
			s, err := asText(x)
			return change(s), err
		}
	}

	return map[string]any{
		"cap":   wrap(capitalize),
		"upper": wrap(func(s string) string { return mapText(s, strings.ToUpper) }),
		"lower": wrap(func(s string) string { return mapText(s, strings.ToLower) }),
	}
}

func asText(x any) (string, error) {
	switch v := x.(type) {
	case string:
		return v, nil
	case htmltemplate.HTML:
		return string(v), nil
	case nil:
		return "", nil
	case Entity:
		return v.Name(), nil
	case fmt.Stringer:
		return v.String(), nil
	default:
		return fmt.Sprint(v), nil
	}
}

// capitalize upper-cases the first letter of s's text.
func capitalize(s string) string {
	done := false
	return mapText(s, func(text string) string {
		if done {
			return text
		}
		for i, r := range text {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				done = true
				return text[:i] + string(unicode.ToUpper(r)) + text[i+utf8.RuneLen(r):]
			}
		}
		return text
	})
}

// protectedRx matches what case changes must leave alone besides color
// codes: HTML tags, HTML entities like &amp;, and the ids inside entity and
// command marks (their labels are text and may change).
var protectedRx = regexp.MustCompile(`<[^>]*>|&[#a-zA-Z0-9]+;|` + markStart + `[^` + markName + `]*` + markName + `|` + commandStart + `[^` + commandLabel + `]*` + commandLabel + `|[` + markEnd + commandEnd + `]`)

// mapText applies change to the parts of s that are text, in order,
// skipping color codes, tags, HTML entities and mark ids.
func mapText(s string, change func(string) string) string {
	spans := append(ansi.CodeSpans(s), protectedRx.FindAllStringIndex(s, -1)...)

	var b strings.Builder
	at := 0
	for at < len(s) {
		next, end := len(s), len(s)
		for _, sp := range spans {
			if sp[0] >= at && sp[0] < next {
				next, end = sp[0], sp[1]
			}
		}
		if next > at {
			b.WriteString(change(s[at:next]))
		}
		if next == len(s) {
			break
		}
		b.WriteString(s[next:end])
		at = end
	}

	return b.String()
}
