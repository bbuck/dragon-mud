package modules_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestModules(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Modules Suite")
}
