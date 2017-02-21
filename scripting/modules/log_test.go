package modules_test

import (
	"github.com/bbuck/dragon-mud/logger"
	"github.com/bbuck/dragon-mud/scripting"
	"github.com/bbuck/dragon-mud/scripting/keys"
	"github.com/bbuck/dragon-mud/scripting/lua"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Log", func() {
	e := lua.NewEngine()
	e.Meta[keys.EngineID] = "test engine"
	e.Meta[keys.Logger] = logger.TestLog()
	scripting.OpenLog(e)
	e.DoString(`log = require("log")`)

	DescribeTable("log module",
		func(script string) {
			logger.TestBuffer.Reset()
			err := e.DoString(script)
			Ω(err).Should(BeNil())
			Ω(logger.TestBuffer.Len()).Should(BeNumerically(">", 0))
		},
		Entry("error() without data", `log.error("Information log 1")`),
		Entry("error() doens't fail", `log.error("Information log 2", nil)`),
		Entry("error() with data", `log.error("Information log 3", {is_test = true})`),
		Entry("warn() without data", `log.warn("Information log 1")`),
		Entry("warn() doens't fail", `log.warn("Information log 2", nil)`),
		Entry("warn() with data", `log.warn("Information log 3", {is_test = true})`),
		Entry("info() without data", `log.info("Information log 1")`),
		Entry("info() doens't fail", `log.info("Information log 2", nil)`),
		Entry("info() with data", `log.info("Information log 3", {is_test = true})`),
		Entry("debug() without data", `log.debug("Information log 1")`),
		Entry("debug() doens't fail", `log.debug("Information log 2", nil)`),
		Entry("debug() with data", `log.debug("Information log 3", {is_test = true})`))
})
