package assets_test

import (
	. "github.com/bbuck/dragon-mud/assets"

	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = DescribeTable("Existence",
	func(assetName string) {
		_, err := Asset(assetName)
		Ω(err).Should(BeNil())
	},
	Entry("Dragonfile.toml", "Dragonfile.toml"),
	Entry("DragonInfo.toml", "DragonInfo.toml"),
	Entry("test.toml", "test.toml"),
	Entry("init.lua", "init.lua"),
	Entry("modules/fn.lua", "modules/fn.lua"))
