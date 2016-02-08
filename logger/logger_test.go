package logger_test

import (
	"bytes"
	"encoding/json"

	"github.com/Sirupsen/logrus"
	"github.com/bbuck/dragon-mud/logger"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Logger", func() {
	var (
		plainStr     = "this is a plain string"
		formatStr    = "*%s*"
		formattedStr = "*" + plainStr + "*"
	)

	logger.Testing = true
	// preload logger
	logger.Log()

	BeforeEach(func() {
		logger.TestBuffer.Reset()
	})

	Context("log wrappers", func() {
		Describe("Info", func() {
			It("logs with the string given", func() {
				logger.Info(plainStr)
				json, err := logResultToJSON(logger.TestBuffer)
				Ω(err).Should(BeNil())
				Ω(json).Should(And(
					HaveKeyWithValue("msg", plainStr),
					HaveKeyWithValue("level", "info"),
				))
			})

			It("logs string with format given", func() {
				logger.Infof(formatStr, plainStr)
				json, err := logResultToJSON(logger.TestBuffer)
				Ω(err).Should(BeNil())
				Ω(json).Should(And(
					HaveKeyWithValue("msg", formattedStr),
					HaveKeyWithValue("level", "info"),
				))
			})
		})

		Describe("Warn", func() {
			It("logs with the string given", func() {
				logger.Warn(plainStr)
				json, err := logResultToJSON(logger.TestBuffer)
				Ω(err).Should(BeNil())
				Ω(json).Should(And(
					HaveKeyWithValue("msg", plainStr),
					HaveKeyWithValue("level", "warning"),
				))
			})

			It("logs string with format given", func() {
				logger.Warnf(formatStr, plainStr)
				json, err := logResultToJSON(logger.TestBuffer)
				Ω(err).Should(BeNil())
				Ω(json).Should(And(
					HaveKeyWithValue("msg", formattedStr),
					HaveKeyWithValue("level", "warning"),
				))
			})
		})

		Describe("Debug", func() {
			It("logs with the string given", func() {
				logger.Debug(plainStr)
				json, err := logResultToJSON(logger.TestBuffer)
				Ω(err).Should(BeNil())
				Ω(json).Should(And(
					HaveKeyWithValue("msg", plainStr),
					HaveKeyWithValue("level", "debug"),
				))
			})

			It("logs string with format given", func() {
				logger.Debugf(formatStr, plainStr)
				json, err := logResultToJSON(logger.TestBuffer)
				Ω(err).Should(BeNil())
				Ω(json).Should(And(
					HaveKeyWithValue("msg", formattedStr),
					HaveKeyWithValue("level", "debug"),
				))
			})
		})

		Describe("Error", func() {
			It("logs with the string given", func() {
				logger.Error(plainStr)
				json, err := logResultToJSON(logger.TestBuffer)
				Ω(err).Should(BeNil())
				Ω(json).Should(And(
					HaveKeyWithValue("msg", plainStr),
					HaveKeyWithValue("level", "error"),
				))
			})

			It("logs string with format given", func() {
				logger.Errorf(formatStr, plainStr)
				json, err := logResultToJSON(logger.TestBuffer)
				Ω(err).Should(BeNil())
				Ω(json).Should(And(
					HaveKeyWithValue("msg", formattedStr),
					HaveKeyWithValue("level", "error"),
				))
			})
		})

		Describe("Panic", func() {
			It("logs with the string given", func() {
				Ω(func() {
					logger.Panic(plainStr)
				}).Should(Panic())
				json, err := logResultToJSON(logger.TestBuffer)
				Ω(err).Should(BeNil())
				Ω(json).Should(And(
					HaveKeyWithValue("msg", plainStr),
					HaveKeyWithValue("level", "panic"),
				))
			})

			It("logs string with format given", func() {
				Ω(func() {
					logger.Panicf(formatStr, plainStr)
				}).Should(Panic())
				json, err := logResultToJSON(logger.TestBuffer)
				Ω(err).Should(BeNil())
				Ω(json).Should(And(
					HaveKeyWithValue("msg", formattedStr),
					HaveKeyWithValue("level", "panic"),
				))
			})
		})

		// Ignoring fatal

		Context("newline loggers", func() {
			BeforeEach(func() {
				log := logger.Log()
				log.Formatter = new(logrus.TextFormatter)
			})

			AfterEach(func() {
				log := logger.Log()
				log.Formatter = new(logrus.JSONFormatter)
			})

			It("Infoln logs with newline", func() {
				logger.Infoln(plainStr)
				str := logger.TestBuffer.String()
				Ω(rune(str[len(str)-1])).Should(Equal('\n'))
			})

			It("Debugln logs with newline", func() {
				logger.Debugln(plainStr)
				str := logger.TestBuffer.String()
				Ω(rune(str[len(str)-1])).Should(Equal('\n'))
			})

			It("Warnln logs with newline", func() {
				logger.Warnln(plainStr)
				str := logger.TestBuffer.String()
				Ω(rune(str[len(str)-1])).Should(Equal('\n'))
			})

			It("Errorln logs with newline", func() {
				logger.Errorln(plainStr)
				str := logger.TestBuffer.String()
				Ω(rune(str[len(str)-1])).Should(Equal('\n'))
			})

			It("Panicln logs with newline", func() {
				Ω(func() {
					logger.Panicln(plainStr)
				}).Should(Panic())
				str := logger.TestBuffer.String()
				Ω(rune(str[len(str)-1])).Should(Equal('\n'))
			})
		})
	})

	Context("with fields wrapper", func() {
		Describe("Plain logs", func() {
			It("info logs with a field", func() {
				logger.WithFields(logrus.Fields{
					"test":  true,
					"other": "other string",
				}).Info(plainStr)
				json, err := logResultToJSON(logger.TestBuffer)
				Ω(err).Should(BeNil())
				Ω(json).Should(And(
					HaveKeyWithValue("msg", plainStr),
					HaveKeyWithValue("level", "info"),
					HaveKeyWithValue("test", true),
					HaveKeyWithValue("other", "other string"),
				))
			})
		})

		Describe("Formatted logs", func() {
			It("info logs with a field", func() {
				logger.WithFields(logrus.Fields{
					"test":  true,
					"other": "other string",
				}).Infof(formatStr, plainStr)
				json, err := logResultToJSON(logger.TestBuffer)
				Ω(err).Should(BeNil())
				Ω(json).Should(And(
					HaveKeyWithValue("msg", formattedStr),
					HaveKeyWithValue("level", "info"),
					HaveKeyWithValue("test", true),
					HaveKeyWithValue("other", "other string"),
				))
			})
		})
	})

	Context("with field wrapper", func() {
		Describe("Plain logs", func() {
			It("info logs with a field", func() {
				logger.WithField("test", true).Info(plainStr)
				json, err := logResultToJSON(logger.TestBuffer)
				Ω(err).Should(BeNil())
				Ω(json).Should(And(
					HaveKeyWithValue("msg", plainStr),
					HaveKeyWithValue("level", "info"),
					HaveKeyWithValue("test", true),
				))
			})
		})

		Describe("Formatted logs", func() {
			It("info logs with a field", func() {
				logger.WithField("test", true).Infof(formatStr, plainStr)
				json, err := logResultToJSON(logger.TestBuffer)
				Ω(err).Should(BeNil())
				Ω(json).Should(And(
					HaveKeyWithValue("msg", formattedStr),
					HaveKeyWithValue("level", "info"),
					HaveKeyWithValue("test", true),
				))
			})
		})
	})
})

func logResultToJSON(buf *bytes.Buffer) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	err := json.NewDecoder(buf).Decode(&result)

	return result, err
}
