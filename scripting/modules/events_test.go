package modules_test

import (
	"github.com/bbuck/dragon-mud/scripting"
	"github.com/bbuck/dragon-mud/scripting/engine"
	"github.com/bbuck/dragon-mud/scripting/pool"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Events Lua Module", func() {
	var (
		p *pool.EnginePool
		c = make(chan int, 1)
	)

	p = pool.NewEnginePool(2, func(e *engine.Lua) {
		e.OpenChannel()
		scripting.OpenEvents(e)

		e.SetGlobal("c", c)
		e.DoString(`
            events = require("events")

            events.on("test1", function(data)
                c:send(1)
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
	}, 10)
})
