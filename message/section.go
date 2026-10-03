package message

import (
	"errors"
	"fmt"
	htmltemplate "html/template"
	"slices"
	"strings"
	"text/template/parse"
)

// Part is what a plugin adds to a message's section: text with color
// codes, or a message kind rendered with data.
type Part struct {
	Text string

	Kind  string
	Block string
	Data  any
}

// SectionFunc returns the parts for the section of the message kind being
// sent. It's called at most once per section per message: parts are the
// same in every format.
type SectionFunc func(kind, section string) ([]Part, error)

// SetSections sets where {{section "name"}} gets its parts. Without it,
// sections are empty.
func (t *Templates) SetSections(f SectionFunc) {
	t.sectionFunc = f
}

// Render modes for parts: how a section's parts are rendered depends on
// the template the section is in.
const (
	modeText = iota
	modeMarked
	modeHTML
)

// section renders {{section "name"}} in a template rendered in mode.
func (t *Templates) section(mode int, name string) (string, error) {
	switch {
	case len(t.rendering) == 0:
		return "", errors.New("section only works while a message is rendering")
	case len(t.rendering) > 1:
		return "", fmt.Errorf("%s can't have sections of its own, because it was added to another message's section", t.rendering[len(t.rendering)-1])
	case t.sectionFunc == nil:
		return "", nil
	}

	parts, err := t.sectionFunc(t.rendering[0], name)
	if err != nil {
		return "", err
	}

	var out []string
	for _, p := range parts {
		s, err := t.renderPart(mode, p)
		if err != nil {
			return "", fmt.Errorf("section %q: %w", name, err)
		}
		if s != "" {
			out = append(out, s)
		}
	}

	if mode == modeHTML {
		return strings.Join(out, ""), nil
	}

	return strings.Join(out, "\n"), nil
}

func (t *Templates) renderPart(mode int, p Part) (string, error) {
	if p.Kind == "" {
		if mode == modeHTML {
			return textToHTML(p.Text), nil
		}
		return p.Text, nil
	}

	t.rendering = append(t.rendering, p.Kind)
	defer func() { t.rendering = t.rendering[:len(t.rendering)-1] }()

	text, ok := t.files[p.Kind][FormatText]
	if !ok {
		return "", fmt.Errorf("there's no message kind %q", p.Kind)
	}

	switch mode {
	case modeText:
		return text.render(text.tmpl, p.Block, p.Data)
	case modeMarked:
		return text.render(text.marked, p.Block, p.Data)
	default:
		if html, ok := t.files[p.Kind][FormatHTML]; ok {
			return html.render(html.tmpl, p.Block, p.Data)
		}
		s, err := text.render(text.marked, p.Block, p.Data)
		return textToHTML(s), err
	}
}

// Sections returns the sections the kind's text template has, in the order
// they first appear.
func (t *Templates) Sections(kind string) []string {
	if c, ok := t.files[kind][FormatText]; ok {
		return c.sections
	}

	return nil
}

// checkSections checks that a kind's HTML template has every section its
// text template has, so parts added to them reach the web.
func (t *Templates) checkSections(name string) error {
	text, hasText := t.files[name][FormatText]
	html, hasHTML := t.files[name][FormatHTML]
	if !hasText || !hasHTML {
		return nil
	}

	for _, s := range text.sections {
		if !slices.Contains(html.sections, s) {
			return fmt.Errorf("%s has no {{section %q}}, but %s does. Add it, so what plugins add to the section reaches the web too.",
				html.path, s, text.path)
		}
	}

	return nil
}

// sectionsIn lists the sections named in trees: calls like
// {{section "exits"}} with a literal name.
func sectionsIn(trees []*parse.Tree) []string {
	var names []string
	var walk func(n parse.Node)
	walk = func(n parse.Node) {
		switch n := n.(type) {
		case *parse.ListNode:
			if n == nil {
				return
			}
			for _, c := range n.Nodes {
				walk(c)
			}
		case *parse.ActionNode:
			walk(n.Pipe)
		case *parse.PipeNode:
			if n == nil {
				return
			}
			for _, c := range n.Cmds {
				walk(c)
			}
		case *parse.CommandNode:
			if len(n.Args) >= 2 {
				if id, ok := n.Args[0].(*parse.IdentifierNode); ok && id.Ident == "section" {
					if s, ok := n.Args[1].(*parse.StringNode); ok && !slices.Contains(names, s.Text) {
						names = append(names, s.Text)
					}
				}
			}
			for _, a := range n.Args {
				walk(a)
			}
		case *parse.IfNode:
			walk(n.Pipe)
			walk(n.List)
			walk(n.ElseList)
		case *parse.RangeNode:
			walk(n.Pipe)
			walk(n.List)
			walk(n.ElseList)
		case *parse.WithNode:
			walk(n.Pipe)
			walk(n.List)
			walk(n.ElseList)
		case *parse.TemplateNode:
			walk(n.Pipe)
		}
	}

	for _, tree := range trees {
		if tree != nil {
			walk(tree.Root)
		}
	}

	return names
}

// sectionHTML is section for HTML templates.
func (t *Templates) sectionHTML(name string) (htmltemplate.HTML, error) {
	s, err := t.section(modeHTML, name)
	return htmltemplate.HTML(s), err
}
