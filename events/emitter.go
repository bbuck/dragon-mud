// Copyright (c) 2016-2017 Brandon Buck

package events

import (
	"fmt"
	"strings"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/bbuck/dragon-mud/logger"
)

// Data is a generic map from strings to any values that can be used as a means
// to wrap a chunk of dynamic data and pass them to event handlers.
// Event data should contain data specific to the event being fired that would
// allow handlers to make actionable response to. Such as an "damage_taken"
// event might have a map containing "source" (who did the damage), "target"
// (who received the damage), and then data about the damage itself.
type Data map[string]interface{}

// NewData returns an empty map[string]interface{} wrapped in the Data type,
// as an easy way to seen event emissions with empty data (where nil would mean
// no data).
func NewData() Data {
	return Data(make(map[string]interface{}))
}

// Handler is a type with a Call function that accepts Data, and represents some
// callable type that wants to perform some action when an event is emitted.
type Handler interface {
	Call(Data) error
}

// HandlerFunc wraps a Go func in a painless way to match the events.Handler
// interface.
type HandlerFunc func(Data) error

// Call will just call the funtion the HandlerFunc type is wrapping and return
// it's results. This allows functions to fit the events.Handler interface
// painlessly.
func (hf HandlerFunc) Call(d Data) error {
	return hf(d)
}

// handlers is a helper type to manage handlers, both calling and adding them.
type handlers struct {
	persistent   []Handler
	onceHandlers []Handler
}

// Iterate over handlers, taking error values from them. On error we break out
// and no longer continue calling handlers. One time handlers that get executed
// before an error alwasy get removed.
func (hs *handlers) call(d Data) error {
	for idx, h := range hs.onceHandlers {
		err := h.Call(d)
		if err != nil {
			if idx != len(hs.onceHandlers)-1 {
				hs.onceHandlers = hs.onceHandlers[idx+1:]
			} else {
				hs.onceHandlers = make([]Handler, 0)
			}

			return err
		}
	}
	for _, h := range hs.persistent {
		err := h.Call(d)
		if err != nil {
			return err
		}
	}

	return nil
}

func (hs *handlers) clear() {
	hs.persistent = make([]Handler, 0)
	hs.onceHandlers = make([]Handler, 0)
}

// Emitter represents a type capable of handling a list of callable actions to
// act on event data.
type Emitter struct {
	handlers map[string]*handlers
	mutex    *sync.RWMutex
	log      *logrus.Entry
}

// NewEmitter generates a new event emitter with the given name used for logging
// purposes.
func NewEmitter(name string) *Emitter {
	return &Emitter{
		handlers: make(map[string]*handlers),
		mutex:    new(sync.RWMutex),
		log:      logger.LogWithSource(fmt.Sprintf("emitter %q", name)),
	}
}

// On registers the handler for the given event.
// Events registered in this manner will be called every time this event is
// emitted.
func (e *Emitter) On(evt string, h Handler) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	if hs, ok := e.handlers[evt]; ok {
		hs.persistent = append(hs.persistent, h)
	} else {
		hs := &handlers{
			persistent:   []Handler{h},
			onceHandlers: make([]Handler, 0),
		}
		e.handlers[evt] = hs
	}
}

// Once resgisters a handler for an event that will fire one time and then
// drop from the handler list.
// This is great for one time handlers, things that don't need to happen
// everytime the event is emitted.
func (e *Emitter) Once(evt string, h Handler) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	if hs, ok := e.handlers[evt]; ok {
		hs.onceHandlers = append(hs.onceHandlers, h)
	} else {
		hs := &handlers{
			persistent:   make([]Handler, 0),
			onceHandlers: []Handler{h},
		}
		e.handlers[evt] = hs
	}
}

// Off will remove all handlers for the given event, including it's before and
// after handlers.
func (e *Emitter) Off(evt string) {
	e.off("before:" + evt)
	e.off(evt)
	e.off("after:" + evt)
}

// clear handlers for event
func (e *Emitter) off(evt string) {
	if hs, ok := e.handlers[evt]; ok {
		hs.clear()
	}
}

// Emit will call all handlers and once handlers assigned to listen to the event
// as well as emitting a before:<event> and after:<event> before and after.
// This method is asyncronous and returns no values directly, failures get
// logged to the log target(s).
func (e *Emitter) Emit(evt string, d Data) {
	if strings.HasPrefix(evt, "before:") || strings.HasPrefix(evt, "after:") {
		e.log.WithFields(logrus.Fields{
			"event": evt,
			"data":  d,
		}).Warn("Cannot emit meta events 'before' or 'after' directly.")
	}

	go func() {
		err := e.emit("before:"+evt, d)
		if err == nil {
			err = e.emit(evt, d)
		}
		if err == nil {
			err = e.emit("after:"+evt, d)
		}

		if err != nil {
			e.log.WithFields(logrus.Fields{
				"error": err.Error(),
				"event": evt,
				"data":  d,
			}).Error("Failed during execution of event handlers.")
		}
	}()
}

// this handles the meat of emitting events, it will iterate over the one time
// handlers and clear out all (or only those that get touched) and then all
// persistent handlers
func (e *Emitter) emit(evt string, d Data) error {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	if hs, ok := e.handlers[evt]; ok {
		return hs.call(d)
	}

	return nil
}
