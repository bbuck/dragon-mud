// Package view renders views: the templates in a plugin's views/ directory,
// one per format, that scripts send by name. It also provides the layout
// helpers and sections templates use. See docs/design.md §4.
package view

import (
	"errors"
	"fmt"
	htmltemplate "html/template"
	"io"
	"maps"
	"regexp"
	"slices"
	"strings"
	texttemplate "text/template"
	"text/template/parse"

	"bbuck.dev/dragon-mud/ansi"
)

// Formats a template can be written in. A template file is named
// <name>.<format>.tmpl.
const (
	// FormatText is text with color codes, for telnet and as the fallback
	// for every other client.
	FormatText = "txt"

	// FormatHTML is HTML for the web client.
	FormatHTML = "html"
)

// Formats lists the formats in the order they're described to authors.
var Formats = []string{FormatText, FormatHTML}

// Entity is an object as templates see it: its id, key and properties.
type Entity map[string]any

// Name is what {{entity .x}} shows for an entity: its name property, else
// its key, else "something".
func (e Entity) Name() string {
	for _, field := range []string{"name", "key"} {
		if s, ok := e[field].(string); ok && s != "" {
			return s
		}
	}

	return "something"
}

// File is one template file a plugin provides.
type File struct {
	// Name is the view's name: "say" for say.txt.tmpl.
	Name string

	// Format is FormatText or FormatHTML.
	Format string

	// Path is where the file is, for errors, such as
	// "dragon:chat/views/say.txt.tmpl".
	Path string

	// Plugin is the plugin that provides it.
	Plugin string

	Source string
}

// Templates holds named templates in each format, such as every view. Files added later replace earlier ones with the same name and
// format, so the game beats plugins beats built-ins one file at a time.
type Templates struct {
	files map[string]map[string]*compiled

	// width is what text is laid out to; see layoutFuncs.
	width int

	sectionFunc SectionFunc

	// rendering is the views being rendered: the one sent, then any part
	// of a section in it.
	rendering []string
}

type compiled struct {
	path   string
	blocks []string
	tmpl   executor

	// marked is a text template whose entities are marked, for rendering
	// it as HTML.
	marked executor

	// sections are the sections the file renders, in order.
	sections []string

	// refs are the fields each template in the file, keyed by name, reads
	// from its data without testing for them.
	refs map[string][]dataRef
}

// executor is a parsed text/template or html/template.
type executor interface {
	Execute(w io.Writer, data any) error
	ExecuteTemplate(w io.Writer, name string, data any) error
}

// NewTemplates returns an empty set of templates.
func NewTemplates() *Templates {
	return &Templates{files: make(map[string]map[string]*compiled)}
}

// SetWidth sets the width text templates lay text out to. Zero means
// DefaultWidth.
func (t *Templates) SetWidth(width int) {
	t.width = width
}

// Width is the width text templates lay text out to.
func (t *Templates) Width() int {
	if t.width <= 0 {
		return DefaultWidth
	}

	return t.width
}

// textFuncs, markedFuncs and htmlFuncs are the functions text templates,
// text templates rendered for HTML, and HTML templates can call.
func (t *Templates) textFuncs() texttemplate.FuncMap {
	funcs := layoutFuncs(textLayout{width: t.Width}, func(s string) string { return s })
	funcs["entity"] = textEntity
	funcs["command"] = textCommand
	funcs["section"] = func(name string) (string, error) { return t.section(modeText, name) }
	return funcs
}

func (t *Templates) markedFuncs() texttemplate.FuncMap {
	funcs := layoutFuncs(markedLayout{}, func(s string) string { return s })
	funcs["entity"] = markedEntity
	funcs["command"] = markedCommand
	funcs["section"] = func(name string) (string, error) { return t.section(modeMarked, name) }
	return funcs
}

func (t *Templates) htmlFuncs() htmltemplate.FuncMap {
	funcs := layoutFuncs(htmlLayout{}, func(s string) htmltemplate.HTML { return htmltemplate.HTML(s) })
	funcs["entity"] = htmlEntity
	funcs["command"] = htmlCommand
	funcs["section"] = t.sectionHTML
	return funcs
}

// Add parses f, replacing any template already added with its name and
// format.
func (t *Templates) Add(f File) error {
	c := &compiled{path: f.Path}

	switch f.Format {
	case FormatText:
		tmpl, err := texttemplate.New(f.Path).Funcs(t.textFuncs()).Parse(f.Source)
		if err != nil {
			return err
		}
		marked, err := tmpl.Clone()
		if err != nil {
			return err
		}
		c.tmpl, c.marked = tmpl, marked.Funcs(t.markedFuncs())
		c.blocks = blocks(f.Path, tmpl.Templates())
		c.refs = make(map[string][]dataRef)
		for _, d := range tmpl.Templates() {
			c.sections = appendNew(c.sections, sectionsIn([]*parse.Tree{d.Tree})...)
			c.refs[d.Name()] = dataRefs(d.Tree)
		}

	case FormatHTML:
		tmpl, err := htmltemplate.New(f.Path).Funcs(t.htmlFuncs()).Parse(f.Source)
		if err != nil {
			return err
		}
		c.tmpl = tmpl
		c.blocks = blocks(f.Path, tmpl.Templates())
		c.refs = make(map[string][]dataRef)
		for _, d := range tmpl.Templates() {
			c.sections = appendNew(c.sections, sectionsIn([]*parse.Tree{d.Tree})...)
			c.refs[d.Name()] = dataRefs(d.Tree)
		}

	default:
		return fmt.Errorf("%s: unknown format %q (expected %s)", f.Path, f.Format, strings.Join(Formats, " or "))
	}

	if t.files[f.Name] == nil {
		t.files[f.Name] = make(map[string]*compiled)
	}
	t.files[f.Name][f.Format] = c

	return nil
}

// named is anything text/template or html/template returns from
// Templates().
type named interface{ Name() string }

// blocks returns the names a file defines, other than the file itself.
func blocks[T named](path string, templates []T) []string {
	var names []string
	for _, tmpl := range templates {
		if tmpl.Name() != path {
			names = append(names, tmpl.Name())
		}
	}
	slices.Sort(names)

	return names
}

// Validate checks that every name has a text template, which every client
// can show. dir is where the files live, such as "views", for the
// error.
func (t *Templates) Validate(dir string) error {
	var errs []error
	for _, name := range t.Names() {
		if _, ok := t.files[name][FormatText]; !ok {
			html := t.files[name][FormatHTML]
			errs = append(errs, fmt.Errorf("%s has no text version. Add %s/%s.txt.tmpl: telnet shows it, and the web does whenever there's no HTML version.",
				html.path, dir, name))
		}
		if err := t.checkSections(name); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// Has reports whether there's a template called name in any format.
func (t *Templates) Has(name string) bool {
	return len(t.files[name]) > 0
}

// Names returns every template's name, sorted.
func (t *Templates) Names() []string {
	return slices.Sorted(maps.Keys(t.files))
}

// Render renders the template name in format with data: the whole file, or
// only block when it isn't empty. Blank lines at either end of the output
// are trimmed. ok is false when name has no template in format, except
// that HTML falls back to the text template, with color and entities as
// HTML.
func (t *Templates) Render(name, format, block string, data any) (out string, ok bool, err error) {
	t.rendering = append(t.rendering, name)
	defer func() { t.rendering = t.rendering[:len(t.rendering)-1] }()

	c, ok := t.files[name][format]
	switch {
	case ok:
		out, err = c.render(c.tmpl, block, data)
		return out, true, err

	case format == FormatHTML:
		if c, ok = t.files[name][FormatText]; ok {
			out, err = c.render(c.marked, block, data)
			return textToHTML(out), true, err
		}
	}

	return "", false, nil
}

func (c *compiled) render(tmpl executor, block string, data any) (string, error) {
	if block != "" && !slices.Contains(c.blocks, block) {
		defines := "It doesn't define any blocks"
		if len(c.blocks) > 0 {
			defines = "It defines " + strings.Join(c.blocks, ", ")
		}
		return "", fmt.Errorf("%s has no block %q. %s; add {{define %q}}...{{end}} to it, or send one of those.",
			c.path, block, defines, block)
	}

	name := block
	if name == "" {
		name = c.path
	}
	if err := checkData(c.refs[name], data); err != nil {
		return "", err
	}

	var b strings.Builder
	var err error
	if block == "" {
		err = tmpl.Execute(&b, data)
	} else {
		err = tmpl.ExecuteTemplate(&b, block, data)
	}
	if err != nil {
		return "", err
	}

	return strings.Trim(b.String(), "\r\n"), nil
}

// Entities in text rendered for HTML are marked with private-use
// characters, which survive escaping, then turned into elements.
const (
	markStart = "\ue000"
	markName  = "\ue001"
	markEnd   = "\ue002"

	commandStart = "\ue003"
	commandLabel = "\ue004"
	commandEnd   = "\ue005"
)

var (
	markRx    = regexp.MustCompile(markStart + "([^" + markName + "]*)" + markName + "([^" + markEnd + "]*)" + markEnd)
	commandRx = regexp.MustCompile(commandStart + "([^" + commandLabel + "]*)" + commandLabel + "([^" + commandEnd + "]*)" + commandEnd)

	unmark = strings.NewReplacer(markStart, "", markName, "", markEnd, "", commandStart, "", commandLabel, "", commandEnd, "")
)

// textToHTML converts rendered text with marked entities and layout to
// HTML.
func textToHTML(text string) string {
	html := ansi.HTML(text)

	// Block elements replace the line breaks around them.
	html = blockMarkRx.ReplaceAllString(html, "$1")
	html = strings.ReplaceAll(html, "\n", "<br>")

	// ansi.HTML has escaped the id and name.
	html = markRx.ReplaceAllString(html, `<dragon-entity ref="$1">$2</dragon-entity>`)
	html = commandRx.ReplaceAllString(html, `<dragon-command value="$1">$2</dragon-command>`)

	return htmlLayout{}.html(html)
}

// textEntity is {{entity .x}} in text: the entity's name.
func textEntity(e any) (string, error) {
	entity, err := asEntity(e)
	if err != nil {
		return "", err
	}

	return entity.Name(), nil
}

// markedEntity is {{entity .x}} in text that will be rendered as HTML.
func markedEntity(e any) (string, error) {
	entity, err := asEntity(e)
	if err != nil {
		return "", err
	}

	id, _ := entity["id"].(string)

	return markStart + unmark.Replace(id) + markName + unmark.Replace(entity.Name()) + markEnd, nil
}

// commandArgs reads {{command "go north" "north"}}: the command, and the
// label to show, which is the command itself when left out.
func commandArgs(command string, label []string) (string, string, error) {
	switch {
	case strings.TrimSpace(command) == "":
		return "", "", errors.New(`command needs the command to run, like {{command "go north" "north"}}`)
	case len(label) > 1:
		return "", "", fmt.Errorf(`command takes the command and one label, like {{command "go north" "north"}}, but got %d labels`, len(label))
	case len(label) == 1:
		return command, label[0], nil
	default:
		return command, command, nil
	}
}

// textCommand is {{command ...}} in text: the label, which is what a
// telnet player reads and types.
func textCommand(command string, label ...string) (string, error) {
	_, text, err := commandArgs(command, label)
	return text, err
}

// markedCommand is {{command ...}} in text that will be rendered as HTML.
func markedCommand(command string, label ...string) (string, error) {
	command, text, err := commandArgs(command, label)
	if err != nil {
		return "", err
	}

	return commandStart + unmark.Replace(command) + commandLabel + unmark.Replace(text) + commandEnd, nil
}

// htmlCommand is {{command ...}} in HTML: a <dragon-command> the web
// client runs as if the player typed the command.
func htmlCommand(command string, label ...string) (htmltemplate.HTML, error) {
	command, text, err := commandArgs(command, label)
	if err != nil {
		return "", err
	}

	return htmltemplate.HTML(fmt.Sprintf(`<dragon-command value="%s">%s</dragon-command>`,
		htmltemplate.HTMLEscapeString(command), htmltemplate.HTMLEscapeString(text))), nil
}

// htmlEntity is {{entity .x}} in HTML: the entity's name as a
// <dragon-entity> the web client makes clickable.
func htmlEntity(e any) (htmltemplate.HTML, error) {
	entity, err := asEntity(e)
	if err != nil {
		return "", err
	}

	id, _ := entity["id"].(string)

	return htmltemplate.HTML(fmt.Sprintf(`<dragon-entity ref="%s">%s</dragon-entity>`,
		htmltemplate.HTMLEscapeString(id), htmltemplate.HTMLEscapeString(entity.Name()))), nil
}

func asEntity(e any) (Entity, error) {
	switch e := e.(type) {
	case Entity:
		return e, nil
	case nil:
		return nil, errors.New("entity needs an object, but got nothing. Check that the data key is spelled the same in the template and in the script that sent it.")
	default:
		return nil, fmt.Errorf("entity needs an object, but got %v (%T)", e, e)
	}
}

// appendNew appends the items list doesn't have yet.
func appendNew(list []string, items ...string) []string {
	for _, item := range items {
		if !slices.Contains(list, item) {
			list = append(list, item)
		}
	}

	return list
}
