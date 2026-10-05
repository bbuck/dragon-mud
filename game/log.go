package game

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"slices"

	"bbuck.dev/dragon-mud/scripting"
)

// pluginGlobals are the globals each plugin's scripts get of their own.
func (g *Game) pluginGlobals(pluginID string) map[string]any {
	return map[string]any{"log": g.logModule(pluginID)}
}

// logModule is the "log" table a plugin's scripts get: it writes to the
// server log, tagged with the plugin.
//
//	log.debug(message[, fields])
//	log.info(message[, fields])
//	log.warn(message[, fields])
//	log.error(message[, fields])
//
// fields is a table of extra values to show, such as { player = p }.
// Objects show as their description.
func (g *Game) logModule(pluginID string) scripting.Module {
	levels := map[string]slog.Level{
		"debug": slog.LevelDebug,
		"info":  slog.LevelInfo,
		"warn":  slog.LevelWarn,
		"error": slog.LevelError,
	}

	module := scripting.Module{Name: "log", Funcs: make(map[string]scripting.Func, len(levels))}
	for name, level := range levels {
		module.Funcs[name] = func(args scripting.Args) (any, error) {
			return nil, g.scriptLog(pluginID, name, level, args)
		}
	}

	return module
}

func (g *Game) scriptLog(pluginID, name string, level slog.Level, args scripting.Args) error {
	message, err := args.String(0)
	if err != nil {
		return fmt.Errorf(`takes a message, like log.%s("placed player", { player = p })`, name)
	}

	attrs := []slog.Attr{slog.String("plugin", pluginID)}
	if args.Len() > 1 && args[1] != nil {
		fields, err := args.Map(1)
		if err != nil {
			return fmt.Errorf(`fields must be a table of names and values, like log.%s("placed player", { player = p })`, name)
		}
		for _, key := range slices.Sorted(maps.Keys(fields)) {
			attrs = append(attrs, slog.Any(key, logValue(fields[key])))
		}
	}
	if args.Len() > 2 {
		return fmt.Errorf(`takes a message and one table of fields, like log.%s("placed player", { player = p })`, name)
	}

	g.log.LogAttrs(context.Background(), level, message, attrs...)

	return nil
}

// logValue makes a script value readable in the log: handles show as
// their description, inside tables too.
func logValue(value any) any {
	switch v := value.(type) {
	case scripting.Handle:
		return v.Describe()
	case scripting.Function:
		return "function"
	case []any:
		list := make([]any, len(v))
		for i, item := range v {
			list[i] = logValue(item)
		}
		return list
	case map[string]any:
		m := make(map[string]any, len(v))
		for key, item := range v {
			m[key] = logValue(item)
		}
		return m
	default:
		return v
	}
}
