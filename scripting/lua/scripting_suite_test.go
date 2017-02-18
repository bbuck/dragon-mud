package lua_test

import (
	"fmt"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestScripting(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Scripting Suite")
}

var (
	fileName     = "test_lua.lua"
	fileContents = `
	function give_me_one()
  		return 1
	end
	`
)

var _ = BeforeSuite(func() {
	file, err := os.Create(fileName)
	if err != nil {
		Fail(err.Error())
	}
	defer file.Close()
	fmt.Fprintln(file, fileContents)
})

var _ = AfterSuite(func() {
	err := os.Remove(fileName)
	if err != nil {
		Fail(err.Error())
	}
})
