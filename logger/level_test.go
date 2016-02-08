package logger_test

import (
	"github.com/Sirupsen/logrus"
	"github.com/bbuck/dragon-mud/logger"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Level", func() {
	Describe("generating log level from string", func() {
		It("should default to debug", func() {
			Ω(logger.GetLogLevel("")).Should(Equal(logrus.DebugLevel))
		})

		It("should choose fatal level", func() {
			Ω(logger.GetLogLevel("fatal")).Should(Equal(logrus.FatalLevel))
		})

		It("should choose panic level", func() {
			Ω(logger.GetLogLevel("panic")).Should(Equal(logrus.PanicLevel))
		})

		It("should choose warn level", func() {
			Ω(logger.GetLogLevel("warn")).Should(Equal(logrus.WarnLevel))
			Ω(logger.GetLogLevel("warning")).Should(Equal(logrus.WarnLevel))
		})

		It("should choose info level", func() {
			Ω(logger.GetLogLevel("info")).Should(Equal(logrus.InfoLevel))
		})

		It("should choose debug level", func() {
			Ω(logger.GetLogLevel("debug")).Should(Equal(logrus.DebugLevel))
		})
	})
})
