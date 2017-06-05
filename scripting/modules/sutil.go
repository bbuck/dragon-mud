package modules

import (
	"regexp"
	"strings"

	"github.com/bbuck/dragon-mud/scripting/lua"
)

var regexpCache = make(map[string]*regexp.Regexp)

// Sutil contains several features that Lua string handling lacks, things like
// joining and regex matching and splitting and trimming and various other
// things.
//   split(input, separator): table
//     @param input: string = the string to perform the split operation on
//     @param separator: string = the separator with which to split the string
//       by
//     split the input string in parts based on matching the separator string
//   join(words, joiner): string
//     @param words: table = list of values that should be joined together
//     @param joiner: string = a string value that should act as the glue
//       between all values in (words) from ealier.
//     combine the input list of strings with the joiner
//   test_rx(needle, haystack): boolean
//     @param needle: pattern = A Go regular expressoin pattern used to test
//       against the given string value?
//     @param haystack: string = the body to perform the search within
//     test the haystack against the needle (regular expression search)
//   starts_with(str, prefix): boolean
//     @param str: string = the value to test against the prefix
//     @param prefix: string = the prefix that is in question
//     determines if the string starts with the given substring
//   ends_with(str, suffix): boolean
//     @param str: string = the value to test against the suffix
//     @param suffix: string = the suffix that is in question
//     determines if the string ends with the given substring
//   contains(haystack, needle): boolean
//     @param haystack: string = the body of data to be searched by the pattern.
//     @param needle: string = the pattern (regular expression) to search for
//       within the text.
//     determines if substring is present in the given string
//   matches(needle, haystack): table
//     @param needle: string = the pattern (regular expression) to compare
//       against the haystack
//     @param haystack: string = the body of data to be compared against the
//       pattern
//     a list of strings that match the needle (regexp)
var Sutil = lua.TableMap{
	"split": func(eng *lua.Engine) int {
		sep := eng.PopString()
		str := eng.PopString()

		strs := strings.Split(str, sep)
		list := eng.NewTable()
		for _, str := range strs {
			list.Append(str)
		}

		eng.PushValue(list)

		return 1
	},
	"join": func(eng *lua.Engine) int {
		joiner := eng.PopString()
		words := eng.PopTable()

		var strs []string
		words.ForEach(func(_ *lua.Value, value *lua.Value) {
			strs = append(strs, value.AsString())
		})

		eng.PushValue(strings.Join(strs, joiner))

		return 1
	},
	"test_rx": func(eng *lua.Engine) int {
		haystack := eng.PopString()
		needle := eng.PopString()

		rx, err := fetchRx(needle)
		if err != nil {
			eng.PushValue(eng.False())

			return 1
		}

		res := rx.MatchString(haystack)
		eng.PushValue(res)

		return 1
	},
	"starts_with": func(eng *lua.Engine) int {
		prefix := eng.PopString()
		str := eng.PopString()

		eng.PushValue(strings.HasPrefix(str, prefix))

		return 1
	},
	"ends_with": func(eng *lua.Engine) int {
		suffix := eng.PopString()
		str := eng.PopString()

		eng.PushValue(strings.HasSuffix(str, suffix))

		return 1
	},
	"contains": func(eng *lua.Engine) int {
		needle := eng.PopString()
		haystack := eng.PopString()

		eng.PushValue(strings.Contains(haystack, needle))

		return 1
	},
	"matches": func(eng *lua.Engine) int {
		haystack := eng.PopString()
		needle := eng.PopString()

		rx, err := fetchRx(needle)
		if err != nil {
			eng.PushValue(eng.NewTable())

			return 1
		}

		res := rx.FindAllString(haystack, -1)

		eng.PushValue(eng.TableFromSlice(res))

		return 1
	},
	"inspect_value": func(eng *lua.Engine) int {
		val := eng.PopValue()
		eng.PushValue(val.Inspect())

		return 1
	},
}

func fetchRx(rx string) (*regexp.Regexp, error) {
	if r, ok := regexpCache[rx]; ok {
		return r, nil
	}

	r, err := regexp.Compile(rx)
	if err == nil {
		regexpCache[rx] = r
	}

	return r, err
}
