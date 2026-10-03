package readline

import (
	"strings"
	"unicode"
)

// ForwardWord returns where Alt+F moves the cursor from p in text.
func ForwardWord(text []rune, p int) int { return forwardWord(text, p) }

// BackwardWord returns where Alt+B moves the cursor from p in text.
func BackwardWord(text []rune, p int) int { return backwardWord(text, p) }

// IsWordRune reports whether readline treats r as part of a word.
func IsWordRune(r rune) bool { return isWordRune(r) }

// isWordRune reports whether r is part of a word for Alt+F, Alt+B, Alt+D and
// friends. Readline only counts letters and digits, so "my-file_name" is
// three words to them. Ctrl+W is different: it splits on whitespace only.
func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

// forwardWord returns where Alt+F lands from p: the end of the next word.
func forwardWord(buf []rune, p int) int {
	for p < len(buf) && !isWordRune(buf[p]) {
		p++
	}
	for p < len(buf) && isWordRune(buf[p]) {
		p++
	}
	return p
}

// backwardWord returns where Alt+B lands from p: the start of this or the
// previous word.
func backwardWord(buf []rune, p int) int {
	for p > 0 && !isWordRune(buf[p-1]) {
		p--
	}
	for p > 0 && isWordRune(buf[p-1]) {
		p--
	}
	return p
}

// backwardBigWord returns where Ctrl+W cuts back to: the previous
// whitespace-separated word.
func backwardBigWord(buf []rune, p int) int {
	for p > 0 && unicode.IsSpace(buf[p-1]) {
		p--
	}
	for p > 0 && !unicode.IsSpace(buf[p-1]) {
		p--
	}
	return p
}

// SplitArgs splits a command line into words roughly the way bash's history
// expansion does: whitespace separates words, quotes and backslashes keep
// spaces inside a word (the quotes stay in the word), and runs of shell
// operators like "&&" or "|" are words of their own.
func SplitArgs(line string) []string {
	var (
		args  []string
		cur   strings.Builder
		quote rune
	)
	flush := func() {
		if cur.Len() > 0 {
			args = append(args, cur.String())
			cur.Reset()
		}
	}
	rs := []rune(line)
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		switch {
		case quote != 0:
			cur.WriteRune(r)
			if r == quote {
				quote = 0
			}
		case r == '\\' && i+1 < len(rs):
			cur.WriteRune(r)
			i++
			cur.WriteRune(rs[i])
		case r == '\'' || r == '"':
			quote = r
			cur.WriteRune(r)
		case unicode.IsSpace(r):
			flush()
		case strings.ContainsRune("|&;<>()", r):
			flush()
			for i < len(rs) && strings.ContainsRune("|&;<>()", rs[i]) {
				cur.WriteRune(rs[i])
				i++
			}
			i--
			flush()
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	return args
}
