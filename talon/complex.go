// Copyright (c) 2016-2017 Brandon Buck

package talon

import (
	"fmt"
	"regexp"
	"strconv"
)

func init() {
	Unmarshalers["C"] = func(bs []byte) (interface{}, error) {
		c := NewComplex(0i)
		cPtr := &c
		err := cPtr.UnmarshalTalon(bs)
		if err != nil {
			return nil, err
		}

		return complex128(c), nil
	}
}

// regular expression defining the structure of a complex value
var complexRx = regexp.MustCompile(`^C!([\d.e+-]+?) \+ ([\d.e+-]+?)i$`)

// the format that will get stored as the field value for complex data types.
const complexStorageFormat = "C!%g + %gi"

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
	switch c := i.(type) {
	case complex64:
		return Complex(complex128(c))
	case *complex64:
		return Complex(complex128(*c))
	case complex128:
		return Complex(c)
	case *complex128:
		return Complex(*c)
	}

	return Complex(0i)
}

// MarshalTalon takes a complex value and turns it into a string representation
// of itself.
func (c Complex) MarshalTalon() ([]byte, error) {
	c128 := complex128(c)
	cStr := fmt.Sprintf(complexStorageFormat, real(c128), imag(c128))

	return []byte(cStr), nil
}

// UnmarshalTalon takes a string and should it parse correctly
func (c *Complex) UnmarshalTalon(bs []byte) error {
	vals := complexRx.FindAllSubmatch(bs, 1)

	if len(vals) == 0 {
		return ComplexParseError("complex string missing correct format")
	}

	rval := string(vals[0][1])
	rp, err := strconv.ParseFloat(rval, 64)
	if err != nil {
		return ComplexParseError(err.Error())
	}

	ival := string(vals[0][2])
	ip, err := strconv.ParseFloat(ival, 64)
	if err != nil {
		return ComplexParseError(err.Error())
	}

	*c = Complex(complex(rp, ip))

	return nil
}
