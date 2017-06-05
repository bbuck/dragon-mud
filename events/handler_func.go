package events

// HandlerFunc wraps a Go func in a painless way to match the events.Handler
// interface.
type HandlerFunc func(Data) error

// Call will just call the funtion the HandlerFunc type is wrapping and return
// it's results. This allows functions to fit the events.Handler interface
// painlessly.
func (hf HandlerFunc) Call(d Data) error {
	return hf(d)
}

// Source returns the pointer to the wrapped HandlerFunc value allowing for
// Go functions to be identified uniquely.
func (hf HandlerFunc) Source() interface{} {
	fn := (func(Data) error)(hf)

	return &fn
}
