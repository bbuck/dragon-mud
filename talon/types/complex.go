// Copyright (c) 2016 Brandon Buck

package types

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// ComplexParseError represents an error that occurred while parsing a complex
// value in string format.
type ComplexParseError string

// Errror returns the message associated with this parse error.
func (cpe ComplexParseError) Error() string {
	return string(cpe)
}

// Complex wraps the native go complex128 type to allow conversion between complex
// values in Go and a string used to store such values in the database.
type Complex complex128

// NewComplex converts the complex64 or complex128 values into a Complex type
// for marshaling but if another type is given this returns 0.
func NewComplex(i interface{}) Complex {
	if c64, ok := i.(complex64); ok {
		return Complex(complex128(c64))
	} else if c128, ok := i.(complex128); ok {
		return Complex(c128)
	}

	return Complex(0i)
}

// MarshalTalon takes a complex value and turns it into a string representation
// of itself.
func (c Complex) MarshalTalon() ([]byte, error) {
	c128 := complex128(c)
	cStr := fmt.Sprintf("%f + %fi", real(c128), imag(c128))

	return []byte(cStr), nil
}

// UnmarshalTalon takes a string and should it parse correctly
func (c *Complex) UnmarshalTalon(bs []byte) error {
	str := string(bs)
	if !strings.Contains(str, " + ") {
		return ComplexParseError("complex string missing correct format")
	}
	if r, _ := utf8.DecodeLastRuneInString(str); r != 'i' {
		return ComplexParseError("complex string does not end with 'i'")
	}
	parts := strings.Split(str, " + ")
	r := parts[0]
	rf, err := strconv.ParseFloat(r, 64)
	if err != nil {
		return err
	}
	if parts[1][len(parts[1])-1] != 'i' {
		return ComplexParseError("complex string didn't end with 'i'")
	}
	im := parts[1][0 : len(parts[1])-1]
	imf, err := strconv.ParseFloat(im, 64)
	if err != nil {
		return err
	}

	*c = Complex(complex(rf, imf))

	return nil
}
