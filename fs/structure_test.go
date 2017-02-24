package fs_test

import (
	"os"
	"path/filepath"

	. "github.com/bbuck/dragon-mud/fs"
	"github.com/bbuck/dragon-mud/logger"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var testStructure = Dir{
	"test.toml": File{},
	"directory": Dir{
		"test.toml": File{},
	},
}

var _ = Describe("Structure", func() {
	var (
		wd   string
		err  error
		cerr error
	)

	BeforeEach(func() {
		wd, err = os.Getwd()
		cerr = CreateFromStructure(CreateStructureParams{
			Log:          logger.TestLog(),
			BaseName:     wd,
			TemplateData: nil,
			Structure:    testStructure,
		})
	})

	AfterEach(func() {
		os.Remove(filepath.Join(wd, "test.toml"))
		os.RemoveAll(filepath.Join(wd, "directory"))
	})

	It("should fetch working directory", func() {
		Ω(err).Should(BeNil())
	})

	It("should not return error for creating structure", func() {
		Ω(cerr).Should(BeNil())
	})

	It("should create test.toml", func() {
		fi, ferr := os.Stat(filepath.Join(wd, "test.toml"))
		Ω(ferr).Should(BeNil())
		Ω(fi.IsDir()).Should(BeFalse())
	})

	It("should create 'directory' as a directory", func() {
		fi, ferr := os.Stat(filepath.Join(wd, "directory"))
		Ω(ferr).Should(BeNil())
		Ω(fi.IsDir()).Should(BeTrue())
	})

	It("should create directory/test.toml", func() {
		fi, ferr := os.Stat(filepath.Join(wd, "directory", "test.toml"))
		Ω(ferr).Should(BeNil())
		Ω(fi.IsDir()).Should(BeFalse())
	})
})
