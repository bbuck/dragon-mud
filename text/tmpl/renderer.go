package tmpl

import "io"

// Renderer is an interface defining the necessary methods for interacting with
// a templtae engine.
type Renderer interface {
	Render(map[string]interface{}) (string, error)
	RenderTo(io.Writer, map[string]interface{}) error
}
