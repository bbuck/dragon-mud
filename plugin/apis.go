package plugin

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"bbuck.dev/dragon-mud/command"
)

// APIs are the APIs a game's plugins provide, each from one plugin. Code
// imports an API by its name, require("@dragon:rooms"), never by the plugin that
// provides it, so a game can swap the provider. See docs/plugins.md.
type APIs struct {
	providers map[string]*Plugin
}

// ResolveAPIs finds the provider of each API and checks that each plugin's
// [uses] are met, from the manifests alone. plugins are in load
// order. engine lists the APIs the built-in plugins provide, whether the
// game loads them or not: the only dragon: APIs there are, which other
// plugins may provide in their place.
func ResolveAPIs(plugins []*Plugin, engine []string) (*APIs, error) {
	a := &APIs{providers: make(map[string]*Plugin)}
	for _, p := range plugins {
		for _, api := range slices.Sorted(maps.Keys(p.Manifest.Provides)) {
			if strings.HasPrefix(api, BuiltinPrefix) && !strings.HasPrefix(p.ID, BuiltinPrefix) && !slices.Contains(engine, api) {
				_, short, _ := strings.Cut(api, ":")
				return nil, fmt.Errorf("%s: %s provides %s, but dragon: is reserved for the engine's APIs, and it has none by that name.%s Name an API for whoever owns its contract, like %s = %q.",
					p.Origin, p.ID, api, command.DidYouMean(api, engine), tomlKey(p.ID+":"+short), p.Manifest.Provides[api].short())
			}
			if other, ok := a.providers[api]; ok {
				return nil, fmt.Errorf("%s and %s both provide the %s API, and a game loads one provider for each API. Load only one of them: drop a built-in from builtins in dragon.toml, or remove a plugin from game/%s/.",
					other.describe(), p.describe(), api, LocalDir)
			}
			a.providers[api] = p
		}
	}

	for _, p := range plugins {
		for _, api := range slices.Sorted(maps.Keys(p.Manifest.Uses)) {
			dep := p.Manifest.Uses[api]
			provider, ok := a.providers[api]
			if !ok {
				if dep.Optional {
					continue
				}
				return nil, fmt.Errorf("%s: %s uses the %s API (%s), but no plugin the game loads provides it.%s Add a plugin that does, or make it optional in %s's [uses]: %s = { version = %q, optional = true }.",
					p.Origin, p.ID, api, dep.Version, suggestAPI(api, slices.Collect(maps.Keys(a.providers))), ManifestFile, tomlKey(api), dep.Version.String())
			}
			if v := provider.Manifest.Provides[api]; !dep.Version.Allows(v) {
				return nil, fmt.Errorf("%s: %s uses the %s API %s, but %s provides %s %s. Use a version of %s that provides a matching one, or change %s's [uses] to %s = %q if it works with %s.",
					p.Origin, p.ID, api, dep.Version, provider.ID, api, v.short(), provider.ID, p.ID, tomlKey(api), caret(v), v.short())
			}
		}
	}

	return a, nil
}

// Provider returns the plugin that provides api to from, which must list it
// in its [uses], unless from is the game's own plugin, which chose what
// loads, or provides api itself. It returns nil if api is an optional API in [uses]
// that no plugin provides.
func (a *APIs) Provider(from *Plugin, api string) (*Plugin, error) {
	provider, provided := a.providers[api]
	dep, uses := from.Manifest.Uses[api]

	switch {
	case !provided && uses && dep.Optional:
		return nil, nil
	case !provided:
		names := slices.Sorted(maps.Keys(a.providers))
		if len(names) == 0 {
			return nil, fmt.Errorf("no plugin the game loads provides the %s API, and none provides any.", api)
		}
		return nil, fmt.Errorf("no plugin the game loads provides the %s API.%s APIs: %s.", api, suggestAPI(api, names), andList(names))
	case from.game || provider == from || uses:
		return provider, nil
	}

	return nil, fmt.Errorf("%s doesn't list the %s API in [uses]. Add it to %s's %s:\n\n[uses]\n%s = %q",
		from.ID, api, from.ID, ManifestFile, tomlKey(api), caret(provider.Manifest.Provides[api]))
}

// suggestAPI suggests the API in names that api may mean: one that's
// spelled almost the same, or the same name in a namespace, like
// dragon:chat for chat.
func suggestAPI(api string, names []string) string {
	slices.Sort(names)
	for _, name := range names {
		if _, short, ok := strings.Cut(name, ":"); ok && short == api {
			return ` Did you mean "` + name + `"?`
		}
	}

	return command.DidYouMean(api, names)
}

// tomlKey writes an API's name as a TOML key: quoted when it has a
// namespace, since TOML's bare keys can't hold a colon.
func tomlKey(api string) string {
	if strings.Contains(api, ":") {
		return strconv.Quote(api)
	}

	return api
}

// describe names the plugin and where it came from, for messages.
func (p *Plugin) describe() string {
	if p.Origin == "" || p.Origin == p.ID {
		return p.ID
	}

	return fmt.Sprintf("%s (%s)", p.ID, p.Origin)
}
