package modules_test

import (
	"fmt"
	"os"
	"strconv"

	"github.com/bbuck/dragon-mud/config"
	"github.com/bbuck/dragon-mud/data"
	"github.com/bbuck/dragon-mud/scripting"
	"github.com/bbuck/dragon-mud/scripting/lua"
	"github.com/spf13/viper"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var performLiveTest bool

// This file contains tests that require an actual connection to a Neo4j instance.
// Communication on authentication/user/pass/host/port details should be done
// via the env variables listed below: (possible values are listed as a list)
//   TALON_PERFORM_LIVE_TEST = [1,0]
//   TALON_LIVE_TEST_AUTHENTICATED = [1,0]
//   TALON_LIVE_TEST_USER = <string>
//   TALON_LIVE_TEST_PASS = <string>
//   TALON_LIVE_TEST_HOST = <string> -- default: localhost
//   TALON_LIVE_TEST_PORT = <uint16>

var _ = Describe("Talon", func() {
	loadLiveTestEnvVariables()

	if !performLiveTest {
		fmt.Println("Skipping live talon module test. Set TALON_PERFORM_LIVE_TEST to 1 to execute live tests.")

		return
	}

	config.Setup(nil)

	var (
		p = lua.NewEnginePool(1, func(eng *lua.Engine) {
			scripting.OpenLibs(eng, "talon")
		})
	)

	Describe("query", func() {
		Context("fetching nodes", func() {
			BeforeEach(func() {
				data.DB().Cypher(`
					CREATE (o:TalonModuleTestNode {name: "first"}),
						   (t:TalonModuleTestNode:OtherLabel {name: "second"}),
						   (th:TalonModuleTestNode {name: "third"}),
						   (o)-[:TALON_MODULE_TEST_REL]->(t),
						   (t)-[:TALON_MODULE_TEST_REL]->(th)
				`).Exec()
			})

			AfterEach(func() {
				data.DB().Cypher(`
					MATCH (n:TalonModuleTestNode)
					OPTIONAL MATCH (n)-[r:TALON_MODULE_TEST_REL]->()
					DELETE n, r
				`).Exec()
			})

			It("fetches a single node", func() {
				By("fetching a node from the script")

				eng := p.Get()
				err := eng.DoString(`
					local talon = require("talon")

					rows = talon.query("MATCH (n:TalonModuleTestNode {name: 'second'}) RETURN n")

					function get_name()
						row = rows:next()
						node = row:get("n")
						str = node:get("name")

						return str, node.labels
					end
				`)

				Ω(err).ShouldNot(HaveOccurred())

				By("get name values from row")

				values, err := eng.Call("get_name", 2)

				Ω(err).ShouldNot(HaveOccurred())
				Ω(values).Should(HaveLen(2))
				Ω(values[0].AsString()).Should(Equal("second"))
				Ω(values[1].Interface()).Should(ConsistOf("TalonModuleTestNode", "OtherLabel"))
			})
		})
	})
})

func loadLiveTestEnvVariables() {
	if val, ok := os.LookupEnv("TALON_PERFORM_LIVE_TEST"); ok {
		performLiveTest = val == "1"
	}

	if val, ok := os.LookupEnv("TALON_LIVE_TEST_AUTHENTICATED"); ok {
		viper.SetDefault("database.development.authentication", val == "1")
	}

	if val, ok := os.LookupEnv("TALON_LIVE_TEST_USER"); ok {
		viper.SetDefault("database.development.username", val)
	}

	if val, ok := os.LookupEnv("TALON_LIVE_TEST_PASS"); ok {
		viper.SetDefault("database.development.password", val)
	}

	if val, ok := os.LookupEnv("TALON_LIVE_TEST_HOST"); ok {
		viper.SetDefault("database.development.host", val)
	}

	if val, ok := os.LookupEnv("TALON_TEST_PORT"); ok {
		if ui, err := strconv.ParseUint(val, 10, 16); err == nil {
			viper.SetDefault("database.development.port", uint16(ui))
		} else {
			fmt.Println("Failed to parse TALON_LIVE_TEST_PORT")
		}
	}
}
