// Copyright (c) 2016-2017 Brandon Buck

package talon_test

import (
	"fmt"
	"io"
	"os"
	"strconv"

	. "github.com/bbuck/talon"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// This file contains tests that require an actual connection to a Neo4j instance.
// Communication on authentication/user/pass/host/port details should be done
// via the env variables listed below: (possible values are listed as a list)
//   TALON_PERFORM_LIVE_TEST = [1,0]
//   TALON_LIVE_TEST_AUTHENTICATED = [1,0]
//   TALON_LIVE_TEST_USER = <string>
//   TALON_LIVE_TEST_PASS = <string>
//   TALON_LIVE_TEST_HOST = <string> -- default: localhost
//   TALON_LIVE_TEST_PORT = <uint16>

const (
	defaultHost        = "localhost"
	defaultUser        = "neo4j"
	defaultPort uint16 = 7687
)

var (
	performLiveTest       bool
	liveTestAuthenticated bool
	liveTestUser          string
	liveTestPassword      string
	liveTestHost          string
	liveTestPort          uint16
)

var _ = Describe("LiveDB", func() {
	loadLiveTestEnvVariables()

	if !performLiveTest {
		fmt.Println("Skipping live test. Set TALON_PERFORM_LIVE_TEST to 1 to execute live tests.")

		return
	}

	co := ConnectOptions{
		User: liveTestUser,
		Host: liveTestHost,
		Port: liveTestPort,
	}

	if liveTestAuthenticated {
		co.Pass = liveTestPassword
	}

	Describe("Connecting", func() {
		var err error

		BeforeEach(func() {
			_, err = co.Connect()
		})

		It("doesn't fail", func() {
			Ω(err).To(BeNil())
		})
	})

	Context("when connected, without a pool", func() {
		var (
			db  *DB
			err error
		)

		BeforeEach(func() {
			db, _ = co.Connect()
		})

		Describe("Cypher/CypherP", func() {
			Context("when making a query", func() {
				BeforeEach(func() {
					_, err = db.Cypher("MATCH (n) RETURN n").Query()
				})

				It("should not fail", func() {
					Ω(err).To(BeNil())
				})
			})

			Context("single node", func() {
				It("allows creating, accessing and deleting", func() {
					By("creating a node")

					result, err := db.Cypher(`CREATE (:TalonSingleNodeTest {hello: "world"})`).Exec()

					Ω(err).ToNot(HaveOccurred())
					Ω(result.Stats.LabelsAdded).To(BeEquivalentTo(1))
					Ω(result.Stats.NodesCreated).To(BeEquivalentTo(1))
					Ω(result.Stats.PropertiesSet).To(BeEquivalentTo(1))

					By("accessing the node")

					rows, err := db.Cypher(`MATCH (n:TalonSingleNodeTest) RETURN n`).Query()
					defer rows.Close()

					Ω(err).ToNot(HaveOccurred())
					Ω(rows).ToNot(BeNil())

					row, err := rows.Next()

					// examine rows
					Ω(err).ToNot(HaveOccurred())
					Ω(row).To(HaveLen(1))
					Ω(row[0].Type()).To(Equal(EntityNode))

					// examine node
					node := row[0].(*Node)
					Ω(node.Labels).To(HaveLen(1))
					Ω(node.Labels).To(ContainElement("TalonSingleNodeTest"))
					Ω(node.Properties).To(HaveLen(1))
					Ω(node.Properties).To(HaveKey("hello"))
					Ω(node.Properties["hello"]).To(Equal("world"))

					row, err = rows.Next()

					Ω(row).To(HaveLen(0))
					Ω(err).To(MatchError(io.EOF))

					By("deleting nodes")

					result, err = db.Cypher("MATCH (n:TalonSingleNodeTest) DELETE n").Exec()

					Ω(err).ToNot(HaveOccurred())
					Ω(result).ToNot(BeNil())
					Ω(result.Stats.NodesDeleted).To(BeEquivalentTo(1))
				})
			})

			Context("single relationship", func() {
				It("allows creating, accessing and deleting", func() {
					By("setting up the database")

					result, err := db.Cypher("CREATE (a:TalonSingleRelTest {id: 1}), (b:TalonSingleRelTest {id: 2})").Exec()

					Ω(err).ToNot(HaveOccurred())
					Ω(result.Stats.NodesCreated).To(BeEquivalentTo(2))

					By("creating relationship")

					result, err = db.Cypher(`MATCH (a:TalonSingleRelTest {id: 1}), (b:TalonSingleRelTest {id: 2}) CREATE (a)-[:TALON_TEST_RELATIONSHIP {hello: "world"}]->(b)`).Exec()

					Ω(err).ToNot(HaveOccurred())
					Ω(result.Stats.RelationshipsCreated).To(BeEquivalentTo(1))

					By("fetching the relationship")

					rows, err := db.Cypher("MATCH ()-[r:TALON_TEST_RELATIONSHIP]->() RETURN r").Query()
					defer rows.Close()

					Ω(err).ToNot(HaveOccurred())
					Ω(rows).ToNot(BeNil())

					row, err := rows.Next()

					Ω(err).ToNot(HaveOccurred())
					Ω(row).To(HaveLen(1))
					Ω(row[0].Type()).To(Equal(EntityRelationship))

					rel := row[0].(*Relationship)
					Ω(rel.Name).To(Equal("TALON_TEST_RELATIONSHIP"))
					Ω(rel.StartNodeID).To(BeNumerically(">", 0))
					Ω(rel.EndNodeID).To(And(
						BeNumerically(">", 0),
						Not(Equal(rel.StartNodeID)),
					))
					Ω(rel.Properties).To(HaveKey("hello"))
					Ω(rel.Properties["hello"]).To(Equal("world"))

					row, err = rows.Next()
					Ω(row).To(HaveLen(0))
					Ω(err).To(MatchError(io.EOF))

					By("deleting the relationship")

					result, err = db.Cypher("MATCH (n)-[r:TALON_TEST_RELATIONSHIP]->(n2) DELETE r, n, n2").Exec()

					Ω(err).ToNot(HaveOccurred())
					Ω(result.Stats.NodesDeleted).To(BeEquivalentTo(2))
					Ω(result.Stats.RelationshipsDeleted).To(BeEquivalentTo(1))
				})
			})
		})
	})
})

func debug(lbl string, v interface{}) {
	fmt.Printf("\n\nDEBUGGING %q ->\n\n%+v\n\n----------\n\n", lbl, v)
}

func loadLiveTestEnvVariables() {
	if val, ok := os.LookupEnv("TALON_PERFORM_LIVE_TEST"); ok {
		performLiveTest = val == "1"
	}

	if val, ok := os.LookupEnv("TALON_LIVE_TEST_AUTHENTICATED"); ok {
		liveTestAuthenticated = val == "1"
	}

	if val, ok := os.LookupEnv("TALON_LIVE_TEST_USER"); ok {
		liveTestUser = val
	} else {
		liveTestUser = defaultUser
	}

	if val, ok := os.LookupEnv("TALON_LIVE_TEST_PASS"); ok {
		liveTestPassword = val
	}

	if val, ok := os.LookupEnv("TALON_LIVE_TEST_HOST"); ok {
		liveTestHost = val
	} else {
		liveTestHost = defaultHost
	}

	if val, ok := os.LookupEnv("TALON_TEST_PORT"); ok {
		if ui, err := strconv.ParseUint(val, 10, 16); err != nil {
			fmt.Println("Failed to parse TALON_LIVE_TEST_PORT")
			liveTestPort = defaultPort
		} else {
			liveTestPort = uint16(ui)
		}
	} else {
		liveTestPort = defaultPort
	}
}
