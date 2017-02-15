// Copyright (c) 2016-2017 Brandon Buck

package talon_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestTalon(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Talon Suite")
}
