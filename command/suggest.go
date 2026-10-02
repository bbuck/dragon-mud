package command

import "strings"

// DidYouMean returns " Did you mean "x"?" when one of options is a likely
// typo of name, or "" when none is close.
func DidYouMean(name string, options []string) string {
	best, bestDist := "", 3
	for _, o := range options {
		if d := distance(strings.ToLower(name), strings.ToLower(o)); d < bestDist && d <= max(1, len(name)/3) {
			best, bestDist = o, d
		}
	}
	if best == "" {
		return ""
	}

	return ` Did you mean "` + best + `"?`
}

// distance is the edit distance between a and b, counting a swap of two
// adjacent letters as one edit, since that's the most common typo.
func distance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	d := make([][]int, len(ra)+1)
	for i := range d {
		d[i] = make([]int, len(rb)+1)
		d[i][0] = i
	}
	for j := range d[0] {
		d[0][j] = j
	}

	for i := 1; i <= len(ra); i++ {
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			d[i][j] = min(d[i-1][j]+1, d[i][j-1]+1, d[i-1][j-1]+cost)
			if i > 1 && j > 1 && ra[i-1] == rb[j-2] && ra[i-2] == rb[j-1] {
				d[i][j] = min(d[i][j], d[i-2][j-2]+1)
			}
		}
	}

	return d[len(ra)][len(rb)]
}
