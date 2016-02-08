package scripting_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestScripting(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Scripting Suite")
}
