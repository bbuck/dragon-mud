// Package hook implements the engine's extension points: the command
// registry, ordered hook chains that can modify or cancel a payload, and
// notifications. Ordering comes from plugin manifests and game-level wiring.
// See docs/design.md §3.
package hook
