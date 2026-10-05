package readline

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// Expand performs bash history expansion on line, the way bash does when
// you press Enter: "!!" is the last command, "!$" its last word, "^a^b"
// reruns it with a replaced by b, and so on. history is oldest first, and
// entry i is number i+1 for "!n".
//
// Supported: !! !n !-n !string !?string? ^old^new, word designators after
// a colon (:0 :n :^ :$ :* :n-m :n* :n-) and the shorthands !^ !$ !* that
// apply to the last command. Modifiers like :s or :h are not supported.
func Expand(line string, history []string) (string, error) {
	rs := []rune(line)
	if len(rs) > 0 && rs[0] == '^' {
		return quickSubst(rs, history)
	}

	var out strings.Builder
	inSingle := false
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		switch {
		case r == '\'' && !inSingle && !insideDouble(rs[:i]):
			inSingle = true
		case r == '\'' && inSingle:
			inSingle = false
		case r == '\\' && !inSingle && i+1 < len(rs):
			out.WriteRune(r)
			i++
			r = rs[i]
		case r == '!' && !inSingle && i+1 < len(rs) && !strings.ContainsRune(" \t\n=(\"", rs[i+1]):
			text, n, err := expandEvent(rs[i:], history)
			if err != nil {
				return "", err
			}
			out.WriteString(text)
			i += n - 1
			continue
		}
		out.WriteRune(r)
	}
	return out.String(), nil
}

// insideDouble reports whether the end of s is inside double quotes.
func insideDouble(s []rune) bool {
	in := false
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case '"':
			in = !in
		}
	}
	return in
}

// expandEvent expands one "!..." at the start of rs, returning the text and
// how many runes it consumed.
func expandEvent(rs []rune, history []string) (string, int, error) {
	i := 1
	var (
		entry string
		err   error
	)
	switch c := rs[i]; {
	case c == '!':
		i++
		entry, err = histEntry(history, len(history), "!!")
	case c == '^' || c == '$' || c == '*' || c == ':':
		// !$ is short for !!:$ and so on.
		entry, err = histEntry(history, len(history), "!!")
	case c == '-' || unicode.IsDigit(c):
		j := i + 1
		for j < len(rs) && unicode.IsDigit(rs[j]) {
			j++
		}
		spec := string(rs[i:j])
		n, convErr := strconv.Atoi(spec)
		if convErr != nil {
			return "", 0, fmt.Errorf("!%s: event not found", spec)
		}
		i = j
		if n < 0 {
			entry, err = histEntry(history, len(history)+n+1, "!"+spec)
		} else {
			entry, err = histEntry(history, n, "!"+spec)
		}
	case c == '?':
		j := i + 1
		for j < len(rs) && rs[j] != '?' {
			j++
		}
		needle := string(rs[i+1 : j])
		i = j
		if j < len(rs) {
			i++ // closing '?'
		}
		entry, err = histFind(history, "!?"+needle, func(h string) bool { return strings.Contains(h, needle) })
	default:
		j := i
		// The string ends at a blank, a word designator or shell syntax.
		for j < len(rs) && !unicode.IsSpace(rs[j]) && !strings.ContainsRune(":^$*%;&|()<>\"'", rs[j]) {
			j++
		}
		prefix := string(rs[i:j])
		i = j
		entry, err = histFind(history, "!"+prefix, func(h string) bool { return strings.HasPrefix(h, prefix) })
	}
	if err != nil {
		return "", 0, err
	}

	// Word designator.
	if i < len(rs) && (rs[i] == ':' || (strings.ContainsRune("^$*", rs[i]) && rs[i-1] != rs[i])) {
		start := i
		if rs[i] == ':' {
			i++
		}
		words := SplitArgs(entry)
		text, n, ok := wordDesignator(rs[i:], words)
		if !ok {
			if rs[start] == ':' && i < len(rs) && !unicode.IsSpace(rs[i]) {
				return "", 0, fmt.Errorf("%s: bad word specifier", string(rs[:i+1]))
			}
			return entry, start, nil
		}
		return text, i + n, nil
	}
	return entry, i, nil
}

// histEntry returns entry number n (1-based).
func histEntry(history []string, n int, spec string) (string, error) {
	if n < 1 || n > len(history) {
		return "", fmt.Errorf("%s: event not found", spec)
	}
	return history[n-1], nil
}

// histFind returns the most recent entry matching.
func histFind(history []string, spec string, match func(string) bool) (string, error) {
	for i := len(history) - 1; i >= 0; i-- {
		if match(history[i]) {
			return history[i], nil
		}
	}
	return "", fmt.Errorf("%s: event not found", spec)
}

// wordDesignator parses ^ $ * n n-m n* n- x-y at the start of rs.
func wordDesignator(rs []rune, words []string) (string, int, bool) {
	if len(rs) == 0 {
		return "", 0, false
	}
	last := len(words) - 1
	join := func(from, to int) (string, bool) {
		if from > to {
			return "", from == to+1 // "n*" past the end is empty, not an error
		}
		if from < 0 || to > last {
			return "", false
		}
		return strings.Join(words[from:to+1], " "), true
	}
	num := func(at int) (int, int, bool) {
		switch {
		case at >= len(rs):
			return 0, 0, false
		case rs[at] == '^':
			return 1, 1, true
		case rs[at] == '$':
			return last, 1, true
		}
		j := at
		for j < len(rs) && unicode.IsDigit(rs[j]) {
			j++
		}
		if j == at {
			return 0, 0, false
		}
		n, _ := strconv.Atoi(string(rs[at:j]))
		return n, j - at, true
	}

	if rs[0] == '*' {
		s, ok := join(1, last)
		return s, 1, ok
	}
	from, n, ok := num(0)
	if !ok {
		return "", 0, false
	}
	switch {
	case n < len(rs) && rs[n] == '*':
		s, ok := join(from, last)
		return s, n + 1, ok
	case n < len(rs) && rs[n] == '-':
		to, m, ok := num(n + 1)
		if !ok { // "n-" means up to but not including the last word
			s, ok := join(from, last-1)
			return s, n + 1, ok
		}
		s, ok := join(from, to)
		return s, n + 1 + m, ok
	}
	s, ok := join(from, from)
	return s, n, ok
}

// quickSubst handles "^old^new^": rerun the last command with the first
// "old" replaced by "new".
func quickSubst(rs []rune, history []string) (string, error) {
	parts := strings.SplitN(string(rs[1:]), "^", 3)
	old := parts[0]
	repl := ""
	if len(parts) > 1 {
		repl = parts[1]
	}
	rest := ""
	if len(parts) > 2 {
		rest = parts[2]
	}
	entry, err := histEntry(history, len(history), "^")
	if err != nil {
		return "", err
	}
	if old == "" || !strings.Contains(entry, old) {
		return "", fmt.Errorf(":s%s%s: substitution failed", "^", old)
	}
	return strings.Replace(entry, old, repl, 1) + rest, nil
}
