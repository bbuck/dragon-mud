package modules_test

import (
	"github.com/bbuck/dragon-mud/scripting"
	"github.com/bbuck/dragon-mud/scripting/lua"
	"github.com/bbuck/dragon-mud/scripting/pool"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Events Lua Module", func() {
	var (
		p *pool.EnginePool
		c = make(chan int, 1)
		d = make(chan int, 1)
	)

	p = pool.NewEnginePool(2, func(e *lua.Engine) {
		e.OpenChannel()
		scripting.OpenEvents(e)

		e.SetGlobal("c", c)
		e.SetGlobal("d", d)
		e.DoString(`
            events = require("events")

            events.on("test1", function(data)
                c:send(1)
            end)

            events.on("test2", function(data)
                d:send(2)
            end)
        `)
	})

	It("handles events that are emitted", func(done Done) {
		eng := p.Get()
		eng.DoString(`events.emit("test1", nil)`)
		eng.Release()

		Ω(<-c).Should(Equal(1))
		close(c)
		close(done)
	})

	It("can be called without a data parameter", func(done Done) {
		eng := p.Get()
		eng.DoString(`events.emit("test2")`)
		eng.Release()

		Ω(<-d).Should(Equal(2))
		close(d)
		close(done)
	})
})
