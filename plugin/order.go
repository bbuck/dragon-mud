package plugin

import (
	"maps"
	"slices"
)

// Order sorts sources, which are in load order, so each plugin loads after
// the plugins providing the APIs it depends on, when they're among
// sources. Plugins with nothing between them keep their order. A plugin
// whose manifest can't be read stays where it is, for Open to report.
// Dependencies that go in a circle keep their order, since plugins can
// still import each other's APIs inside functions.
func Order(sources []Source) []Source {
	manifests := make([]Manifest, len(sources))
	providers := make(map[string]int)
	for i, src := range sources {
		m, err := ReadManifest(src.Files)
		if err != nil {
			continue
		}
		manifests[i] = m
		for api := range m.Provides {
			if _, ok := providers[api]; !ok {
				providers[api] = i
			}
		}
	}

	sorted := make([]Source, 0, len(sources))
	state := make([]int, len(sources)) // 0 unvisited, 1 visiting, 2 done
	var visit func(i int)
	visit = func(i int) {
		if state[i] != 0 {
			return
		}
		state[i] = 1
		for _, api := range slices.Sorted(maps.Keys(manifests[i].Depends)) {
			if j, ok := providers[api]; ok && j != i {
				visit(j)
			}
		}
		state[i] = 2
		sorted = append(sorted, sources[i])
	}
	for i := range sources {
		visit(i)
	}

	return sorted
}
