package fs_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestFs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Fs Suite")
}
