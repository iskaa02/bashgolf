package game

// KeyInfo describes the shortcuts levels can introduce.
var KeyInfo = map[string]string{
	"ctrl+a":        "jump to the start of the line",
	"ctrl+e":        "jump to the end of the line",
	"ctrl+f":        "forward one character",
	"ctrl+b":        "back one character",
	"alt+f":         "forward to the end of the word",
	"alt+b":         "back to the start of the word",
	"ctrl+x ctrl+x": "jump between the cursor and the mark (the line start, or where you last pasted)",
	"ctrl+u":        "cut everything before the cursor",
	"ctrl+k":        "cut everything after the cursor",
	"ctrl+w":        "cut the word before the cursor (up to whitespace)",
	"alt+d":         "cut the word after the cursor (stops at punctuation)",
	"alt+backspace": "cut the word before the cursor (stops at punctuation)",
	"ctrl+d":        "delete the character under the cursor",
	"ctrl+h":        "delete the character before the cursor (Backspace)",
	"ctrl+y":        "paste the last thing you cut",
	"alt+y":         "right after Ctrl+Y: swap the paste for the cut before it",
	"ctrl+_":        "undo",
	"alt+r":         "undo everything on this line",
	"alt+u":         "UPPERCASE to the end of the word",
	"alt+l":         "lowercase to the end of the word",
	"alt+c":         "Capitalize the word",
	"ctrl+t":        "swap the two characters around the cursor",
	"alt+t":         "swap the two words around the cursor",
	"ctrl+p":        "previous command in history (same as ↑)",
	"ctrl+n":        "next command in history (same as ↓)",
	"ctrl+r":        "search back through history as you type; Ctrl+R again for older matches",
	"ctrl+s":        "search forward through history (the other way from Ctrl+R)",
	"ctrl+g":        "give up on a search and get your line back",
	"alt+.":         "insert the last argument of the previous command",

	// History expansion: typed text that bash rewrites when you press Enter.
	"!!":       "the whole previous command, e.g. sudo !!",
	"!$":       "the last argument of the previous command",
	"!string":  "the latest command starting with string, e.g. !vim",
	"!n":       "command number n from `history`, e.g. !42",
	"^old^new": "rerun the previous command with old replaced by new",
}

// typedPattern says what a history-expansion "key" looks like in typed
// text, so tests can check a level's solution really uses it.
var typedPattern = map[string]string{
	"!!":       `!!`,
	"!$":       `!\$`,
	"!string":  `![A-Za-z]`,
	"!n":       `![0-9]`,
	"^old^new": `^\^[^^]+\^`,
}

// baseKeys are always available to the par solver: the basic moves anyone
// already knows from arrow keys, Backspace and Delete.
var baseKeys = []string{"ctrl+f", "ctrl+b", "ctrl+h", "ctrl+d"}
