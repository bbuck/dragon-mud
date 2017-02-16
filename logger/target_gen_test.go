package logger_test

import (
	"os"

	"github.com/bbuck/dragon-mud/logger"
	"github.com/bbuck/dragon-mud/output"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TargetGen", func() {
	var (
		complexMap = []map[string]interface{}{
			{
				"type":   "os",
				"target": "stdout",
			},
			{
				"type":   "file",
				"target": "logger.log",
			},
		}
		simpleMap = []map[string]interface{}{
			{
				"type":   "os",
				"target": "stderr",
			},
		}
	)

	Context("nil targets", func() {
		It("should return a console stdout reference", func() {
			writer := logger.ConfigureTargets(nil)
			Ω(writer).To(Equal(output.Stdout()))
		})
	})

	Context("with invalid interface object", func() {
		It("should panic", func() {
			Ω(func() {
				logger.ConfigureTargets(make(map[string]interface{}))
			}).To(Panic())
		})
	})

	Context("with a single target defined", func() {
		It("should return a single io.Writer", func() {
			writer := logger.ConfigureTargets(simpleMap)
			Ω(writer).To(Equal(output.Stderr()))
		})
	})

	Context("with multiple targets", func() {
		AfterEach(func() {
			os.Remove("logger.log")
		})

		It("should create the writers", func() {
			Ω(func() {
				logger.ConfigureTargets(complexMap)
			}).ToNot(Panic())
			info, err := os.Stat("logger.log")
			Ω(err).To(BeNil())
			Ω(info.Name()).To(Equal("logger.log"))
		})
	})
})
