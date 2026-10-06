package plugin

import (
	"fmt"
	"strconv"
	"strings"
)

// Version is an API's version: major, minor and patch. "1.2" is 1.2.0.
type Version struct {
	Major, Minor, Patch int
}

// ParseVersion reads a version of one to three numbers, like "1", "1.2" or
// "1.2.3".
func ParseVersion(s string) (Version, error) {
	v, _, err := parseVersion(s)

	return v, err
}

// parseVersion returns the version in s and how many of its numbers were
// written.
func parseVersion(s string) (Version, int, error) {
	parts := strings.Split(s, ".")
	if s == "" || len(parts) > 3 {
		return Version{}, 0, fmt.Errorf("%q isn't a version. Use one to three numbers, like \"1.2\" or \"1.2.3\".", s)
	}
	var nums [3]int
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 || part != strconv.Itoa(n) {
			return Version{}, 0, fmt.Errorf("%q isn't a version. Use one to three numbers, like \"1.2\" or \"1.2.3\".", s)
		}
		nums[i] = n
	}

	return Version{nums[0], nums[1], nums[2]}, len(parts), nil
}

func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// short is v without trailing zero parts past the minor, as a manifest
// would write it: 1.2 for 1.2.0.
func (v Version) short() string {
	if v.Patch == 0 {
		return fmt.Sprintf("%d.%d", v.Major, v.Minor)
	}

	return v.String()
}

// Less reports whether v is older than o.
func (v Version) Less(o Version) bool {
	return v.less(o)
}

func (v Version) less(o Version) bool {
	if v.Major != o.Major {
		return v.Major < o.Major
	}
	if v.Minor != o.Minor {
		return v.Minor < o.Minor
	}

	return v.Patch < o.Patch
}

// Constraint is the versions of an API a plugin accepts, as Cargo writes
// them: "^1.2" (or "1.2") is 1.2 up to 2.0, "~1.2" is 1.2 up to 1.3 and
// "=1.2.3" is that version only. Below 1.0, a caret accepts only versions
// with the same minor (^0.2 is 0.2 up to 0.3), since each minor may break.
type Constraint struct {
	text     string
	min, max Version
}

// ParseConstraint reads a version constraint.
func ParseConstraint(s string) (Constraint, error) {
	op, rest := "^", s
	for _, prefix := range []string{"^", "~", "="} {
		if r, ok := strings.CutPrefix(s, prefix); ok {
			op, rest = prefix, strings.TrimSpace(r)
			break
		}
	}

	v, parts, err := parseVersion(rest)
	if err != nil {
		return Constraint{}, fmt.Errorf("%q isn't a version constraint. Use \"^1.2\" for 1.2 up to 2.0, \"~1.2\" for 1.2 up to 1.3, or \"=1.2.3\" for exactly 1.2.3.", s)
	}

	c := Constraint{text: s, min: v}
	switch op {
	case "=":
		c.max = Version{v.Major, v.Minor, v.Patch + 1}
	case "~":
		if parts == 1 {
			c.max = Version{v.Major + 1, 0, 0}
		} else {
			c.max = Version{v.Major, v.Minor + 1, 0}
		}
	default:
		switch {
		case v.Major > 0 || parts == 1:
			c.max = Version{v.Major + 1, 0, 0}
		case v.Minor > 0 || parts == 2:
			c.max = Version{0, v.Minor + 1, 0}
		default:
			c.max = Version{0, 0, v.Patch + 1}
		}
	}

	return c, nil
}

// Allows reports whether v meets the constraint.
func (c Constraint) Allows(v Version) bool {
	return !v.less(c.min) && v.less(c.max)
}

func (c Constraint) String() string {
	return c.text
}

// caret is the constraint a plugin would write to accept v and what's
// compatible with it, for suggestions: ^1.2 for 1.2.0.
func caret(v Version) string {
	return "^" + v.short()
}
