package scripting

import (
	"strings"
	"testing"
)

func TestArgsInt(t *testing.T) {
	args := Args{3.0, int64(4), 1.5, "5"}

	if n, err := args.Int(0); err != nil || n != 3 {
		t.Errorf("Int(0) = %d, %v; want 3", n, err)
	}
	if n, err := args.Int(1); err != nil || n != 4 {
		t.Errorf("Int(1) = %d, %v; want 4", n, err)
	}
	if _, err := args.Int(2); err == nil {
		t.Error("Int(2) should reject 1.5 rather than truncate it")
	}
	if _, err := args.Int(3); err == nil {
		t.Error("Int(3) should reject a string")
	}
}

func TestArgsErrors(t *testing.T) {
	args := Args{1.0}

	_, err := args.String(0)
	if err == nil || err.Error() != "argument #1: expected string, got number" {
		t.Errorf("String(0) error = %v", err)
	}

	_, err = args.Map(1)
	if err == nil || !strings.Contains(err.Error(), "argument #2: expected map, got nothing") {
		t.Errorf("Map(1) error = %v", err)
	}
}

func TestArgsTruthy(t *testing.T) {
	args := Args{nil, false, true, 0.0, ""}
	want := []bool{false, false, true, true, true, false}

	for i, w := range want {
		if got := args.Truthy(i); got != w {
			t.Errorf("Truthy(%d) = %v, want %v", i, got, w)
		}
	}
}
