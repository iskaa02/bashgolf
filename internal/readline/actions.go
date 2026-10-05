// Package readline is a simulated GNU Readline line editor. It has no UI
// code: callers feed it key names (spelled the way Bubble Tea spells them,
// e.g. "ctrl+a", "alt+f") and render its state however they like.
package readline

// Action is a readline command, like "beginning-of-line" or "yank".
type Action int

const (
	ActNone Action = iota
	ActSelfInsert

	// Navigation
	ActBeginningOfLine
	ActEndOfLine
	ActForwardChar
	ActBackwardChar
	ActForwardWord
	ActBackwardWord
	ActExchangePointAndMark

	// Killing, deleting and yanking
	ActUnixLineDiscard
	ActKillLine
	ActUnixWordRubout
	ActKillWord
	ActBackwardKillWord
	ActDeleteChar
	ActBackwardDeleteChar
	ActYank
	ActYankPop
	ActUndo
	ActRevertLine

	// Case and transposition
	ActUpcaseWord
	ActDowncaseWord
	ActCapitalizeWord
	ActTransposeChars
	ActTransposeWords

	// History
	ActPreviousHistory
	ActNextHistory
	ActYankLastArg
	ActReverseSearch
	ActForwardSearch

	ActAcceptLine
)

// actionNames are the names readline itself uses (see `bind -l` in bash).
var actionNames = map[Action]string{
	ActNone:                 "",
	ActSelfInsert:           "self-insert",
	ActBeginningOfLine:      "beginning-of-line",
	ActEndOfLine:            "end-of-line",
	ActForwardChar:          "forward-char",
	ActBackwardChar:         "backward-char",
	ActForwardWord:          "forward-word",
	ActBackwardWord:         "backward-word",
	ActExchangePointAndMark: "exchange-point-and-mark",
	ActUnixLineDiscard:      "unix-line-discard",
	ActKillLine:             "kill-line",
	ActUnixWordRubout:       "unix-word-rubout",
	ActKillWord:             "kill-word",
	ActBackwardKillWord:     "backward-kill-word",
	ActDeleteChar:           "delete-char",
	ActBackwardDeleteChar:   "backward-delete-char",
	ActYank:                 "yank",
	ActYankPop:              "yank-pop",
	ActUndo:                 "undo",
	ActRevertLine:           "revert-line",
	ActUpcaseWord:           "upcase-word",
	ActDowncaseWord:         "downcase-word",
	ActCapitalizeWord:       "capitalize-word",
	ActTransposeChars:       "transpose-chars",
	ActTransposeWords:       "transpose-words",
	ActPreviousHistory:      "previous-history",
	ActNextHistory:          "next-history",
	ActYankLastArg:          "yank-last-arg",
	ActReverseSearch:        "reverse-search-history",
	ActForwardSearch:        "forward-search-history",
	ActAcceptLine:           "accept-line",
}

func (a Action) String() string { return actionNames[a] }

// IsMovement reports whether a only moves the cursor.
func (a Action) IsMovement() bool {
	switch a {
	case ActBeginningOfLine, ActEndOfLine, ActForwardChar, ActBackwardChar,
		ActForwardWord, ActBackwardWord, ActExchangePointAndMark:
		return true
	}
	return false
}

func isKill(a Action) bool {
	switch a {
	case ActUnixLineDiscard, ActKillLine, ActUnixWordRubout, ActKillWord, ActBackwardKillWord:
		return true
	}
	return false
}

// Keymap is bash's default emacs keymap, restricted to what we simulate.
var Keymap = map[string]Action{
	"ctrl+a": ActBeginningOfLine,
	"home":   ActBeginningOfLine,
	"ctrl+e": ActEndOfLine,
	"end":    ActEndOfLine,
	"ctrl+f": ActForwardChar,
	"right":  ActForwardChar,
	"ctrl+b": ActBackwardChar,
	"left":   ActBackwardChar,
	"alt+f":  ActForwardWord,
	"alt+b":  ActBackwardWord,

	"ctrl+u":        ActUnixLineDiscard,
	"ctrl+k":        ActKillLine,
	"ctrl+w":        ActUnixWordRubout,
	"alt+d":         ActKillWord,
	"alt+backspace": ActBackwardKillWord,
	"ctrl+d":        ActDeleteChar,
	"delete":        ActDeleteChar,
	"ctrl+h":        ActBackwardDeleteChar,
	"backspace":     ActBackwardDeleteChar,
	"ctrl+y":        ActYank,
	"alt+y":         ActYankPop,
	"ctrl+_":        ActUndo,
	"alt+r":         ActRevertLine,

	"alt+u":  ActUpcaseWord,
	"alt+l":  ActDowncaseWord,
	"alt+c":  ActCapitalizeWord,
	"ctrl+t": ActTransposeChars,
	"alt+t":  ActTransposeWords,

	"ctrl+p": ActPreviousHistory,
	"up":     ActPreviousHistory,
	"ctrl+n": ActNextHistory,
	"down":   ActNextHistory,
	"alt+.":  ActYankLastArg,
	"alt+_":  ActYankLastArg,
	"ctrl+r": ActReverseSearch,
	"ctrl+s": ActForwardSearch,

	"enter":  ActAcceptLine,
	"ctrl+j": ActAcceptLine,
}

// prefixKeymaps hold two-key chords such as Ctrl+X Ctrl+X.
var prefixKeymaps = map[string]map[string]Action{
	"ctrl+x": {
		"ctrl+x": ActExchangePointAndMark,
		"ctrl+u": ActUndo,
	},
}
