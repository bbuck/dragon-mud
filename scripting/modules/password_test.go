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
				local params, success = password.getRandomParams()
				if not success then
					return ""
				end
				local hash, success = password.hash("this is a password", params)
				if not success then
					return ""
				end
				local match = password.isValid("this is a password", hash, params)
				local notMatch = password.isValid("this isn't a password", hash, params)
				return hash, match, notMatch
			end
		`
	)

	config.RegisterDefaults()
	e = lua.NewEngine()
	scripting.OpenPassword(e)
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

	It("generates the correct hash length", func() {
		Ω(result).Should(HaveLen(32))
	})

	It("generates the same hash with matching inputs", func() {
		Ω(valid).Should(BeTrue())
	})

	It("hashes different passwords, differently", func() {
		Ω(invalid).Should(BeFalse())
	})
})
