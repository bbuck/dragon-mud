package modules_test

import (
	"github.com/bbuck/dragon-mud/config"
	"github.com/bbuck/dragon-mud/scripting"
	"github.com/bbuck/dragon-mud/scripting/lua"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("password Module", func() {
	var (
		e       *lua.Engine
		values  []*lua.Value
		result  string
		valid   bool
		invalid bool
		err     error
		script  = `
			local password = require("password")

			function testCrypto()
				local hash = password.hash("this is a password")
				local match = password.is_valid("this is a password", hash)
				local notMatch = password.is_valid("this isn't a password", hash)

				return hash, match, notMatch
			end
		`
	)

	config.RegisterDefaults()
	e = lua.NewEngine()
	scripting.OpenLibs(e, "password")
	e.DoString(script)

	BeforeEach(func() {
		values, err = e.Call("testCrypto", 3)
		if err == nil {
			result = values[0].AsString()
			valid = values[1].AsBool()
			invalid = values[2].AsBool()
		}
	})

	It("doesn't fail", func() {
		Ω(err).Should(BeNil())
	})

	It("doesn't generate empty strings", func() {
		Ω(result).ShouldNot(Equal(""))
	})

	It("generates the same hash with matching inputs", func() {
		Ω(valid).Should(BeTrue())
	})

	It("hashes different passwords, differently", func() {
		Ω(invalid).Should(BeFalse())
	})
})
