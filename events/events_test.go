package events_test

import (
	. "github.com/bbuck/dragon-mud/events"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Events", func() {
	Describe("Emitter", func() {
		em := NewEmitter("testing")

		It("receives emitted events", func(done Done) {
			c := make(chan interface{})
			em.On("test1", HandlerFunc(func(Data) error {
				c <- true

				return nil
			}))

			em.Emit("test1", nil)

			Ω(<-c).To(Equal(true))
			close(c)
			close(done)
		})

		It("receives before and after emitted events", func(done Done) {
			c := make(chan interface{})
			em.On("before:test2", HandlerFunc(func(Data) error {
				c <- 1

				return nil
			}))

			em.On("test2", HandlerFunc(func(Data) error {
				c <- 2

				return nil
			}))

			em.On("after:test2", HandlerFunc(func(Data) error {
				c <- 3

				return nil
			}))

			em.Emit("test2", nil)

			Ω(<-c).To(Equal(1))
			Ω(<-c).To(Equal(2))
			Ω(<-c).To(Equal(3))
			close(c)
			close(done)
		})

		It("transfers altered data", func(done Done) {
			c := make(chan interface{})
			em.On("before:test3", HandlerFunc(func(d Data) error {
				d["one"] = int(1)

				return nil
			}))

			em.On("test3", HandlerFunc(func(d Data) error {
				if val, ok := d["one"]; ok {
					if num, ok := val.(int); ok {
						c <- num
					} else {
						c <- 10
					}
				} else {
					c <- 11
				}

				d["two"] = int(2)

				return nil
			}))

			em.On("test3", HandlerFunc(func(d Data) error {
				if val, ok := d["two"]; ok {
					if num, ok := val.(int); ok {
						c <- num
					} else {
						c <- 12
					}
				} else {
					c <- 13
				}

				d["three"] = int(3)

				return nil
			}))

			em.On("after:test3", HandlerFunc(func(d Data) error {
				if val, ok := d["three"]; ok {
					if num, ok := val.(int); ok {
						c <- num
					} else {
						c <- 14
					}
				} else {
					c <- 15
				}

				return nil
			}))

			em.Emit("test3", NewData())

			Ω(<-c).To(Equal(1))
			Ω(<-c).To(Equal(2))
			Ω(<-c).To(Equal(3))
			close(c)
			close(done)
		})

		It("only fires once handlers one time", func(done Done) {
			c := make(chan interface{})
			em.Once("test4", HandlerFunc(func(Data) error {
				c <- true

				return nil
			}))

			em.Emit("test4", nil)
			Ω(<-c).To(Equal(true))

			// close and emit again, a panic from writing to closed channel is
			// the failure we're looking for.
			close(c)
			em.Emit("test4", nil)

			close(done)
		})

		It("stops execution if an error is returned", func(done Done) {
			c := make(chan interface{})
			em.On("test5", HandlerFunc(func(Data) error {
				c <- 1
				close(c)

				return ErrHalt
			}))

			em.On("test5", HandlerFunc(func(Data) error {
				c <- 2

				return nil
			}))

			em.Emit("test5", nil)
			Ω(<-c).To(Equal(1))
			close(done)
		})

	})
})
