package tmpl_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestTmpl(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Tmpl Suite")
}
