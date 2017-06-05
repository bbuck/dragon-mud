package modules_test

import (
	"github.com/bbuck/dragon-mud/events"
	"github.com/bbuck/dragon-mud/logger"
	"github.com/bbuck/dragon-mud/scripting"
	"github.com/bbuck/dragon-mud/scripting/lua"

	"github.com/bbuck/dragon-mud/scripting/keys"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Events Lua Module", func() {
	var (
		p *lua.EnginePool
		c = make(chan int, 1)
		d = make(chan int, 1)
		f = make(chan int, 1)
		g = make(chan int, 1)
		h = make(chan int, 1)
	)

	em := events.NewEmitter(logger.New().WithField("note", "external_emitter"))

	p = pool.NewEnginePool(2, func(e *lua.Engine) {
		e.Meta[keys.ExternalEmitter] = em

		e.OpenChannel()
		scripting.OpenLibs(e, "events")

		e.SetGlobal("c", c)
		e.SetGlobal("d", d)
		e.SetGlobal("f", f)
		e.SetGlobal("g", g)
		e.SetGlobal("h", h)
		e.DoString(`
			events = require("events")

			events.on("test1", function(data)
				c:send(1)
			end)

			events.on("test2", function(data)
				d:send(2)
			end)

			events.on("test3", function(data)
				f:send(3)
			end)

			events.once("test4", function(data)
				g:send(4)
			end)

			events.on("emit_once_setup", function(data)
				events.on("test5", function(data)
					h:send(5)
				end)
			end)
        `)
	})

	It("handles events that are emitted", func(done Done) {
		eng := p.Get()
		eng.DoString(`events.emit("test1", nil)`)
		eng.Release()

		Ω(<-c).Should(Equal(1))
		Consistently(func() int {
			return len(c)
		}).ShouldNot(BeNumerically(">", 0))
		close(c)
		close(done)
	})

	It("can be called without a data parameter", func(done Done) {
		eng := p.Get()
		eng.DoString(`events.emit("test2")`)
		eng.Release()

		Ω(<-d).Should(Equal(2))
		Consistently(func() int {
			return len(d)
		}).ShouldNot(BeNumerically(">", 0))
		close(d)
		close(done)
	})

	It("can be triggered from outside of the engine", func(done Done) {
		em.Emit("test3", nil)

		Ω(<-f).Should(Equal(3))
		Consistently(func() int {
			return len(f)
		}).ShouldNot(BeNumerically(">", 0))
		close(f)
		close(done)
	})

	It("handles calling events only once", func(done Done) {
		em.Emit("test4", nil)
		em.Emit("test4", nil)

		Ω(<-g).Should(Equal(4))
		Consistently(func() int {
			return len(g)
		}).ShouldNot(BeNumerically(">", 0))
		close(g)
		close(done)
	})

	It("handles emit once events properly", func(done Done) {
		d := em.EmitOnce("test5", nil)
		<-d
		em.Emit("emit_once_setup", nil)

		Ω(<-h).Should(Equal(5))
		Consistently(func() int {
			return len(h)
		}).ShouldNot(BeNumerically(">", 0))
		close(h)
		close(done)
	})
})
