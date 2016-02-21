package ansi_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestColor(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ANSI Suite")
}
