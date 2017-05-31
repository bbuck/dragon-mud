// Copyright (c) 2016-2017 Brandon Buck

package events

import "sync"

// handlers is a helper type to manage handlers, both calling and adding them.
type handlers struct {
	persistent   []Handler
	onceHandlers []Handler
	mutex        *sync.RWMutex
}

func newHandlers() *handlers {
	return &handlers{
		persistent:   make([]Handler, 0),
		onceHandlers: make([]Handler, 0),
		mutex:        new(sync.RWMutex),
	}
}

// Iterate over handlers, taking error values from them. On error we break out
// and no longer continue calling handlers. One time handlers that get executed
// before an error alwasy get removed.
func (hs *handlers) call(d Data) error {
	err := hs.fireOnceHandlers(d)
	if err != nil {
		return err
	}

	err = hs.firePersistentHandlers(d)

	return err
}

func (hs *handlers) firePersistentHandlers(d Data) error {
	hs.mutex.RLock()
	defer hs.mutex.RUnlock()
	for _, h := range hs.persistent {
		err := h.Call(d)
		if err != nil {
			return err
		}
	}

	return nil
}

func (hs *handlers) fireOnceHandlers(d Data) error {
	var (
		idx int
		h   Handler
	)

	hs.mutex.RLock()
	for idx, h = range hs.onceHandlers {
		err := h.Call(d)
		if err != nil {
			hs.mutex.RUnlock()
			hs.mutex.Lock()
			if idx != len(hs.onceHandlers)-1 {
				hs.onceHandlers = hs.onceHandlers[idx+1:]
			} else {
				hs.onceHandlers = make([]Handler, 0)
			}
			hs.mutex.Unlock()

			return err
		}
	}
	hs.mutex.RUnlock()
	hs.mutex.Lock()
	defer hs.mutex.Unlock()
	hs.onceHandlers = make([]Handler, 0)

	return nil
}

func (hs *handlers) add(h Handler) {
	hs.mutex.Lock()
	defer hs.mutex.Unlock()
	for _, oh := range hs.persistent {
		if oh.Source() == h.Source() {
			return
		}
	}

	hs.persistent = append(hs.persistent, h)
}

func (hs *handlers) addOnce(h Handler) {
	hs.mutex.Lock()
	defer hs.mutex.Unlock()
	for _, oh := range hs.onceHandlers {
		if oh.Source() == h.Source() {
			return
		}
	}

	hs.onceHandlers = append(hs.onceHandlers, h)
}

// remove all handlers
func (hs *handlers) clear() {
	hs.mutex.Lock()
	hs.mutex.Unlock()
	hs.persistent = make([]Handler, 0)
	hs.onceHandlers = make([]Handler, 0)
}
