// Copyright (c) 2016-2017 Brandon Buck

package tmpl

import "io"

// Renderer is an interface defining the necessary methods for interacting with
// a templtae engine.
type Renderer interface {
	Render(interface{}) (string, error)
	RenderTo(io.Writer, interface{}) error
}
