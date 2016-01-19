package output_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestOutput(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Output Suite")
}
