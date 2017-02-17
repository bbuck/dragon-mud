package modules_test

import (
	"github.com/bbuck/dragon-mud/scripting"
	"github.com/bbuck/dragon-mud/scripting/engine"
	"github.com/bbuck/dragon-mud/scripting/keys"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Log", func() {
	e := engine.NewLua()
	e.SetGlobal(keys.EngineID, "test engine")
	scripting.OpenLog(e)
	e.DoString(`log = require("log")`)

	Describe("error()", func() {
		Context("without data", func() {
			It("doens't fail", func() {
				err := e.DoString(`log.error("Information log 1")`)
				Ω(err).Should(BeNil())
			})
		})

		Context("with nil data", func() {
			It("doens't fail", func() {
				err := e.DoString(`log.error("Information log 2", nil)`)
				Ω(err).Should(BeNil())
			})
		})

		Context("with data", func() {
			It("doens't fail", func() {
				err := e.DoString(`log.error("Information log 3", {is_test = true})`)
				Ω(err).Should(BeNil())
			})
		})
	})

	Describe("warn()", func() {
		Context("without data", func() {
			It("doens't fail", func() {
				err := e.DoString(`log.warn("Information log 1")`)
				Ω(err).Should(BeNil())
			})
		})

		Context("with nil data", func() {
			It("doens't fail", func() {
				err := e.DoString(`log.warn("Information log 2", nil)`)
				Ω(err).Should(BeNil())
			})
		})

		Context("with data", func() {
			It("doens't fail", func() {
				err := e.DoString(`log.warn("Information log 3", {is_test = true})`)
				Ω(err).Should(BeNil())
			})
		})
	})

	Describe("info()", func() {
		Context("without data", func() {
			It("doens't fail", func() {
				err := e.DoString(`log.info("Information log 1")`)
				Ω(err).Should(BeNil())
			})
		})

		Context("with nil data", func() {
			It("doens't fail", func() {
				err := e.DoString(`log.info("Information log 2", nil)`)
				Ω(err).Should(BeNil())
			})
		})

		Context("with data", func() {
			It("doens't fail", func() {
				err := e.DoString(`log.info("Information log 3", {is_test = true})`)
				Ω(err).Should(BeNil())
			})
		})
	})

	Describe("debug()", func() {
		Context("without data", func() {
			It("doens't fail", func() {
				err := e.DoString(`log.debug("Information log 1")`)
				Ω(err).Should(BeNil())
			})
		})

		Context("with nil data", func() {
			It("doens't fail", func() {
				err := e.DoString(`log.debug("Information log 2", nil)`)
				Ω(err).Should(BeNil())
			})
		})

		Context("with data", func() {
			It("doens't fail", func() {
				err := e.DoString(`log.debug("Information log 3", {is_test = true})`)
				Ω(err).Should(BeNil())
			})
		})
	})
})
