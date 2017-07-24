package utils

import "fmt"

// WTFIsThis is a debug fucntion for development where it prints a detailed
// value of the object.
func WTFIsThis(val interface{}) {
	fmt.Printf("\n\n%+v\n\n", val)
}

// WTFAreIsThis is a debug function for development where it prints a detailed
// value of the object along with it's type.
func WTFAreIsThis(val interface{}) {
	fmt.Printf("\n\n%T\n\n%+v\n\n", val, val)
}

// WTFAreYou is a development debug function that prints the type of something.
func WTFAreYou(val interface{}) {
	fmt.Printf("\n\n%T\n\n", val)
}
