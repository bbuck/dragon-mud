// Copyright (c) 2016-2017 Brandon Buck

package types_test

import (
	"time"

	. "github.com/bbuck/talon/types"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TimeType", func() {
	var (
		bs   []byte
		err  error
		t    Time
		date = time.Date(1986, time.November, 12, 1, 2, 3, 4, time.Local)
	)

	BeforeEach(func() {
		bs = make([]byte, 0)
		err = nil
	})

	Describe("MarshalTalon", func() {
		Context("with default format", func() {
			var test = date.Format(DefaultTimeFormat)

			BeforeEach(func() {
				t = NewTime(date)
				bs, err = t.MarshalTalon()
			})

			It("doesn't fail", func() {
				Ω(err).To(BeNil())
			})

			It("produces the correct string", func() {
				Ω(string(bs)).To(Equal(test))
			})
		})

		Context("with a non-default format", func() {
			var test = date.Format(time.ANSIC)

			BeforeEach(func() {
				t = NewTimeWithFormat(date, time.ANSIC)
				bs, err = t.MarshalTalon()
			})

			It("doesn't fail", func() {
				Ω(err).To(BeNil())
			})

			It("produces the correct string", func() {
				Ω(string(bs)).To(Equal(test))
			})
		})
	})

	Describe("UnmarshalTalon", func() {
		Context("with default format", func() {
			var test = date.Format(DefaultTimeFormat)

			BeforeEach(func() {
				t = EmptyTime()
				err = t.UnmarshalTalon([]byte(test))
			})

			It("doesn't fail", func() {
				Ω(err).To(BeNil())
			})

			It("parsed correct year", func() {
				Ω(t.Year()).To(Equal(1986))
			})

			It("parsed correct month", func() {
				Ω(t.Month()).To(Equal(time.November))
			})

			It("parsed the correct day", func() {
				Ω(t.Day()).To(Equal(12))
			})

			It("parsed the correct hour", func() {
				Ω(t.Hour()).To(Equal(1))
			})

			It("parsed the correct minute", func() {
				Ω(t.Minute()).To(Equal(2))
			})

			It("parsed the correct second", func() {
				Ω(t.Second()).To(Equal(3))
			})
		})

		Context("with a non-default format", func() {
			var test = date.Format(time.ANSIC)

			BeforeEach(func() {
				t = EmptyTimeWithFormat(time.ANSIC)
				err = t.UnmarshalTalon([]byte(test))
			})

			It("doesn't fail", func() {
				Ω(err).To(BeNil())
			})

			It("parsed correct year", func() {
				Ω(t.Year()).To(Equal(1986))
			})

			It("parsed correct month", func() {
				Ω(t.Month()).To(Equal(time.November))
			})

			It("parsed the correct day", func() {
				Ω(t.Day()).To(Equal(12))
			})

			It("parsed the correct hour", func() {
				Ω(t.Hour()).To(Equal(1))
			})

			It("parsed the correct minute", func() {
				Ω(t.Minute()).To(Equal(2))
			})

			It("parsed the correct second", func() {
				Ω(t.Second()).To(Equal(3))
			})
		})
	})
})
