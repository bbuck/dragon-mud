package command

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// maxSplits bounds how many ways one pattern may split one input, so a
// pattern with many free-text slots can't make parsing slow.
const maxSplits = 256

// Token is one word of input, or a quoted string.
type Token struct {
	// Text is the word, or a quoted string's contents.
	Text string

	// Start and End are the token's byte offsets in the input, including
	// any quotes.
	Start, End int

	// Quoted tokens never match literal words.
	Quoted bool
}

// Tokenize splits input into words. Text in double quotes is one token, so
// players can always say exactly what they mean: say "hi to bob". An
// unclosed quote runs to the end of the input. A leading punctuation
// character is its own token (see ParsePattern).
func Tokenize(input string) []Token {
	var tokens []Token

	for i := 0; i < len(input); {
		r, size := utf8.DecodeRuneInString(input[i:])
		switch {
		case unicode.IsSpace(r):
			i += size

		case r == '"':
			end := strings.IndexByte(input[i+1:], '"')
			if end < 0 {
				tokens = append(tokens, Token{Text: input[i+1:], Start: i, End: len(input), Quoted: true})
				i = len(input)
				continue
			}
			end += i + 1
			tokens = append(tokens, Token{Text: input[i+1 : end], Start: i, End: end + 1, Quoted: true})
			i = end + 1

		case len(tokens) == 0 && isCommandPunct(r):
			tokens = append(tokens, Token{Text: string(r), Start: i, End: i + size})
			i += size

		default:
			start := i
			for i < len(input) {
				r, size := utf8.DecodeRuneInString(input[i:])
				if unicode.IsSpace(r) || r == '"' {
					break
				}
				i += size
			}
			tokens = append(tokens, Token{Text: input[start:i], Start: start, End: i})
		}
	}

	return tokens
}

// span is the tokens a slot covers, [from, to).
type span struct{ from, to int }

// text returns the input a span covers. A single quoted token gives its
// contents; anything else gives the input as typed, spacing included.
func spanText(input string, tokens []Token, s span) string {
	if s.to-s.from == 1 && tokens[s.from].Quoted {
		return tokens[s.from].Text
	}

	return input[tokens[s.from].Start:tokens[s.to-1].End]
}

// match returns every way p matches tokens, as one span per slot in
// pattern order. Splits are ordered earliest first: the first slot that can
// vary takes as few tokens as possible.
func (p Pattern) match(tokens []Token, single func(*Slot) bool) [][]span {
	var results [][]span
	var spans []span

	var walk func(ei, ti int)
	walk = func(ei, ti int) {
		if len(results) >= maxSplits {
			return
		}
		if ei == len(p.Elements) {
			if ti == len(tokens) {
				results = append(results, append([]span(nil), spans...))
			}
			return
		}

		e := p.Elements[ei]
		if e.Slot == nil {
			if ti < len(tokens) && !tokens[ti].Quoted && strings.EqualFold(tokens[ti].Text, e.Literal) {
				walk(ei+1, ti+1)
			}
			return
		}

		// Every later element needs at least one token.
		maxEnd := len(tokens) - (len(p.Elements) - ei - 1)
		if single(e.Slot) {
			maxEnd = min(maxEnd, ti+1)
		}
		for end := ti + 1; end <= maxEnd; end++ {
			spans = append(spans, span{ti, end})
			walk(ei+1, end)
			spans = spans[:len(spans)-1]
		}
	}
	walk(0, 0)

	return results
}
