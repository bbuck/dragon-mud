package view

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"text/template/parse"
)

// dataRef is a field a template reads straight from the data it's given,
// such as .room in {{.room.name}}, where nothing tests for it first.
type dataRef struct {
	key string

	// at is where the template reads it, as "path:line:col".
	at string

	// expr is the expression, such as ".room.name".
	expr string
}

// dataRefs finds the fields tree reads from the data without testing for
// them. A field tested by an enclosing if or with, or in the condition of
// an if, with or range, may be missing: that's how templates show
// optional parts. Inside with and range, dot is something else, so only
// $-rooted fields count there.
func dataRefs(tree *parse.Tree) []dataRef {
	if tree == nil || tree.Root == nil {
		return nil
	}

	w := refWalker{tree: tree}
	w.list(tree.Root, true, nil)

	return w.refs
}

type refWalker struct {
	tree *parse.Tree
	refs []dataRef
}

func (w *refWalker) list(l *parse.ListNode, dotIsData bool, tested map[string]bool) {
	if l == nil {
		return
	}
	for _, n := range l.Nodes {
		w.node(n, dotIsData, tested)
	}
}

func (w *refWalker) node(n parse.Node, dotIsData bool, tested map[string]bool) {
	switch n := n.(type) {
	case *parse.ActionNode:
		w.pipe(n.Pipe, dotIsData, tested)
	case *parse.IfNode:
		inner := w.tests(n.Pipe, dotIsData, tested)
		w.list(n.List, dotIsData, inner)
		w.list(n.ElseList, dotIsData, tested)
	case *parse.WithNode:
		inner := w.tests(n.Pipe, dotIsData, tested)
		w.list(n.List, false, inner)
		w.list(n.ElseList, dotIsData, tested)
	case *parse.RangeNode:
		inner := w.tests(n.Pipe, dotIsData, tested)
		w.list(n.List, false, inner)
		w.list(n.ElseList, dotIsData, tested)
	case *parse.TemplateNode:
		w.pipe(n.Pipe, dotIsData, tested)
	case *parse.ListNode:
		w.list(n, dotIsData, tested)
	}
}

// tests returns tested plus the data fields pipe mentions, which the
// pipe's body may then treat as optional. The pipe itself may read them
// even if they're missing.
func (w *refWalker) tests(pipe *parse.PipeNode, dotIsData bool, tested map[string]bool) map[string]bool {
	inner := maps.Clone(tested)
	if inner == nil {
		inner = make(map[string]bool)
	}

	probe := refWalker{tree: w.tree}
	probe.pipe(pipe, dotIsData, nil)
	for _, r := range probe.refs {
		inner[r.key] = true
	}

	return inner
}

func (w *refWalker) pipe(pipe *parse.PipeNode, dotIsData bool, tested map[string]bool) {
	if pipe == nil {
		return
	}
	for _, cmd := range pipe.Cmds {
		for _, arg := range cmd.Args {
			w.arg(arg, dotIsData, tested)
		}
	}
}

func (w *refWalker) arg(n parse.Node, dotIsData bool, tested map[string]bool) {
	switch n := n.(type) {
	case *parse.FieldNode:
		if dotIsData {
			w.add(n, n.Ident, tested)
		}
	case *parse.VariableNode:
		if len(n.Ident) > 1 && n.Ident[0] == "$" {
			w.add(n, n.Ident[1:], tested)
		}
	case *parse.ChainNode:
		w.arg(n.Node, dotIsData, tested)
	case *parse.PipeNode:
		w.pipe(n, dotIsData, tested)
	}
}

func (w *refWalker) add(n parse.Node, idents []string, tested map[string]bool) {
	if len(idents) == 0 || tested[idents[0]] {
		return
	}

	at, _ := w.tree.ErrorContext(n)
	w.refs = append(w.refs, dataRef{key: idents[0], at: at, expr: n.String()})
}

// checkData reports the first field refs reads that data doesn't have.
// Only maps are checked; anything else is left to the template.
func checkData(refs []dataRef, data any) error {
	m, ok := data.(map[string]any)
	if !ok {
		return nil
	}

	for _, r := range refs {
		if _, ok := m[r.key]; ok {
			continue
		}

		has := "The data is empty"
		if len(m) > 0 {
			has = "The data has " + strings.Join(slices.Sorted(maps.Keys(m)), ", ")
		}
		in := ""
		if r.expr != "."+r.key && r.expr != "$."+r.key {
			in = " (in " + r.expr + ")"
		}
		return fmt.Errorf("%s: .%s isn't in the data%s. %s. Send %s with the view, check the template for a typo, or wrap it in {{if .%s}}...{{end}} if it's optional.",
			r.at, r.key, in, has, r.key, r.key)
	}

	return nil
}
