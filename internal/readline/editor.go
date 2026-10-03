package readline

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
)

// Key is one keypress. Name is the Bubble Tea spelling ("ctrl+w", "alt+.",
// "backspace"); Text is set only for plain typing or a paste.
type Key struct {
	Name string
	Text string
}

// Event tells the caller about outcomes beyond the line changing.
type Event int

const (
	EventNone   Event = iota
	EventDing         // not possible here; readline would ring the bell
	EventEOF          // Ctrl+D on an empty line: bash would exit
	EventAccept       // Enter
)

// Result describes what one Feed call did.
type Result struct {
	Action  Action
	Event   Event
	Pending bool // the key was a chord prefix (Ctrl+X) waiting for more
	Unbound bool // the key does nothing in readline
}

// snapshot is the line before an edit. point is where undo leaves the
// cursor; -1 means work it out from what changed (see restore).
type snapshot struct {
	buf   []rune
	point int
}

// Editor is one simulated readline session: a line being edited plus the
// kill ring and history, which survive across lines just like in bash.
type Editor struct {
	buf   []rune
	point int
	mark  int

	kills KillRing
	undo  []snapshot
	last  Action

	// Where the last yank or yank-last-arg put its text, so Alt+Y and
	// repeated Alt+. can replace it.
	yankStart, yankEnd int
	lastArgCount       int

	history []string
	histPos int    // len(history) means the line being typed, not an entry
	saved   []rune // the line being typed, kept while browsing history

	prefix string // pending chord prefix, e.g. "ctrl+x"
}

// New returns an editor holding text with the cursor at point.
func New(text string, point int) *Editor {
	e := &Editor{}
	e.Reset(text, point)
	return e
}

// Reset starts a fresh line. The kill ring and history are kept.
func (e *Editor) Reset(text string, point int) {
	e.buf = []rune(text)
	e.point = max(0, min(point, len(e.buf)))
	e.mark = 0
	e.undo = nil
	e.last = ActNone
	e.histPos = len(e.history)
	e.saved = nil
	e.prefix = ""
}

// SetKillRing replaces the kill ring, oldest entry first. Ctrl+Y will paste
// the last one.
func (e *Editor) SetKillRing(entries []string) {
	e.kills = KillRing{}
	for _, k := range entries {
		e.kills.add([]rune(k), false, false)
	}
}

// SetHistory replaces the history, oldest entry first.
func (e *Editor) SetHistory(h []string) {
	e.history = slices.Clone(h)
	e.histPos = len(e.history)
	e.saved = nil
}

func (e *Editor) Text() string        { return string(e.buf) }
func (e *Editor) Point() int          { return e.point }
func (e *Editor) Mark() int           { return e.mark }
func (e *Editor) Pending() string     { return e.prefix }
func (e *Editor) KillRing() *KillRing { return &e.kills }
func (e *Editor) History() []string   { return e.history }
func (e *Editor) HistoryPos() int     { return e.histPos }

// Clone returns an independent copy, kill ring and undo history included.
func (e *Editor) Clone() *Editor {
	c := *e
	c.buf = slices.Clone(e.buf)
	c.undo = slices.Clone(e.undo)
	c.history = slices.Clone(e.history)
	c.saved = slices.Clone(e.saved)
	c.kills.entries = make([][]rune, len(e.kills.entries))
	for i, k := range e.kills.entries {
		c.kills.entries[i] = slices.Clone(k)
	}
	return &c
}

// Fingerprint identifies everything that affects how the editor reacts to
// future keys, apart from undo and history. Two editors with the same
// fingerprint are interchangeable for a search over editing keys.
//
// A search that never presses Alt+Y can pass wholeRing=false: then only the
// entry Ctrl+Y would paste matters. Likewise mark=false if it never
// presses Ctrl+X Ctrl+X. Leaving those out shrinks the search a lot.
func (e *Editor) Fingerprint(wholeRing, mark bool) string {
	var b strings.Builder
	b.WriteString(string(e.buf))
	fmt.Fprintf(&b, "\x00%d\x00%s", e.point, e.prefix)
	if mark {
		fmt.Fprintf(&b, "\x00m%d", e.mark)
	}
	if wholeRing {
		fmt.Fprintf(&b, "\x00%d", e.kills.idx)
		for _, k := range e.kills.entries {
			b.WriteString("\x01" + string(k))
		}
	} else {
		b.WriteString("\x01" + string(e.kills.current()))
	}
	switch {
	case isKill(e.last):
		b.WriteString("\x00kill")
	case wholeRing && (e.last == ActYank || e.last == ActYankPop):
		fmt.Fprintf(&b, "\x00yank%d,%d", e.yankStart, e.yankEnd)
	case e.last == ActYankLastArg:
		fmt.Fprintf(&b, "\x00lastarg%d,%d,%d", e.yankStart, e.yankEnd, e.lastArgCount)
	}
	return b.String()
}

// Submit returns the current line, records it in history (unless blank) and
// starts a new empty line, like pressing Enter in bash.
func (e *Editor) Submit() string {
	line := e.Text()
	if strings.TrimSpace(line) != "" {
		e.history = append(e.history, line)
	}
	e.Reset("", 0)
	return line
}

// Feed processes one keypress.
func (e *Editor) Feed(k Key) Result {
	if e.prefix != "" {
		chord := prefixKeymaps[e.prefix]
		e.prefix = ""
		a, ok := chord[k.Name]
		if !ok {
			e.last = ActNone
			return Result{Event: EventDing, Unbound: true}
		}
		return Result{Action: a, Event: e.Exec(a)}
	}
	if _, ok := prefixKeymaps[k.Name]; ok {
		e.prefix = k.Name
		return Result{Pending: true}
	}
	if a, ok := Keymap[k.Name]; ok {
		return Result{Action: a, Event: e.Exec(a)}
	}
	if k.Text != "" {
		return Result{Action: ActSelfInsert, Event: e.Insert(k.Text)}
	}
	return Result{Unbound: true}
}

// Exec runs one action.
func (e *Editor) Exec(a Action) Event { return e.apply(a, "") }

// Insert types text at the cursor.
func (e *Editor) Insert(text string) Event { return e.apply(ActSelfInsert, text) }

func (e *Editor) apply(a Action, text string) Event {
	before := snapshot{slices.Clone(e.buf), -1}
	if a == ActTransposeChars {
		// Readline undoes a transpose as delete+insert, ending at the swap.
		before.point = e.point
		if before.point == len(e.buf) {
			before.point--
		}
	}
	ev := e.run(a, text)

	// Record an undo step for anything that changed the text. A run of
	// typed characters is one step, so Ctrl+_ removes a whole typed word.
	changed := !slices.Equal(before.buf, e.buf)
	switch {
	case a == ActUndo, a == ActRevertLine, a == ActPreviousHistory, a == ActNextHistory:
	case changed && !(a == ActSelfInsert && e.last == ActSelfInsert):
		e.undo = append(e.undo, before)
	}
	e.last = a
	return ev
}

func (e *Editor) run(a Action, text string) Event {
	n := len(e.buf)
	switch a {
	case ActSelfInsert:
		e.insert([]rune(text))

	case ActBeginningOfLine:
		e.point = 0
	case ActEndOfLine:
		e.point = n
	case ActForwardChar:
		if e.point == n {
			return EventDing
		}
		e.point++
	case ActBackwardChar:
		if e.point == 0 {
			return EventDing
		}
		e.point--
	case ActForwardWord:
		e.point = forwardWord(e.buf, e.point)
	case ActBackwardWord:
		e.point = backwardWord(e.buf, e.point)
	case ActExchangePointAndMark:
		if e.mark > n {
			e.mark = n
			return EventDing
		}
		e.point, e.mark = e.mark, e.point

	case ActUnixLineDiscard:
		if e.point == 0 {
			return EventDing
		}
		e.kill(e.point, 0)
	case ActKillLine:
		e.kill(e.point, n)
	case ActUnixWordRubout:
		if e.point == 0 {
			return EventDing
		}
		e.kill(e.point, backwardBigWord(e.buf, e.point))
	case ActKillWord:
		if e.point == n {
			return EventDing
		}
		e.kill(e.point, forwardWord(e.buf, e.point))
	case ActBackwardKillWord:
		if e.point == 0 {
			return EventDing
		}
		e.kill(e.point, backwardWord(e.buf, e.point))
	case ActDeleteChar:
		if n == 0 {
			return EventEOF
		}
		if e.point == n {
			return EventDing
		}
		e.buf = slices.Delete(e.buf, e.point, e.point+1)
	case ActBackwardDeleteChar:
		if e.point == 0 {
			return EventDing
		}
		e.buf = slices.Delete(e.buf, e.point-1, e.point)
		e.point--

	case ActYank:
		text := e.kills.current()
		if text == nil {
			return EventDing
		}
		e.mark = e.point
		e.yankStart = e.point
		e.insert(text)
		e.yankEnd = e.point
	case ActYankPop:
		if (e.last != ActYank && e.last != ActYankPop) || e.kills.current() == nil {
			return EventDing
		}
		e.buf = slices.Delete(e.buf, e.yankStart, e.yankEnd)
		e.point = e.yankStart
		e.kills.rotate()
		e.insert(e.kills.current())
		e.yankEnd = e.point
	case ActUndo:
		if len(e.undo) == 0 {
			return EventDing
		}
		e.restore(e.undo[len(e.undo)-1])
		e.undo = e.undo[:len(e.undo)-1]
	case ActRevertLine:
		if len(e.undo) == 0 {
			return EventDing
		}
		e.restore(e.undo[0])
		e.undo = nil

	case ActUpcaseWord:
		e.changeCase(unicode.ToUpper)
	case ActDowncaseWord:
		e.changeCase(unicode.ToLower)
	case ActCapitalizeWord:
		e.changeCase(nil)
	case ActTransposeChars:
		if e.point == 0 || n < 2 {
			return EventDing
		}
		if e.point == n {
			e.point--
		}
		e.buf[e.point-1], e.buf[e.point] = e.buf[e.point], e.buf[e.point-1]
		e.point++
	case ActTransposeWords:
		return e.transposeWords()

	case ActPreviousHistory:
		if e.histPos == 0 {
			return EventDing
		}
		if e.histPos == len(e.history) {
			e.saved = slices.Clone(e.buf)
		}
		e.histPos--
		e.loadLine([]rune(e.history[e.histPos]))
	case ActNextHistory:
		if e.histPos >= len(e.history) {
			return EventDing
		}
		e.histPos++
		if e.histPos == len(e.history) {
			e.loadLine(e.saved)
		} else {
			e.loadLine([]rune(e.history[e.histPos]))
		}
	case ActYankLastArg:
		return e.yankLastArg()

	case ActAcceptLine:
		return EventAccept
	}
	return EventNone
}

func (e *Editor) insert(r []rune) {
	e.buf = slices.Insert(e.buf, e.point, r...)
	e.point += len(r)
}

// restore puts back an undo snapshot. Like readline, the cursor lands at the
// end of the region that changed: after text that comes back, or where
// removed text used to start.
func (e *Editor) restore(s snapshot) {
	prefix := 0
	for prefix < len(s.buf) && prefix < len(e.buf) && s.buf[prefix] == e.buf[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < min(len(s.buf), len(e.buf))-prefix &&
		s.buf[len(s.buf)-1-suffix] == e.buf[len(e.buf)-1-suffix] {
		suffix++
	}
	e.buf = slices.Clone(s.buf) // edits happen in place; keep snapshots intact
	e.point = len(s.buf) - suffix
	if s.point >= 0 {
		e.point = s.point
	}
}

// kill cuts the text between from and to into the kill ring. from > to
// means a backward kill (Ctrl+U, Ctrl+W).
func (e *Editor) kill(from, to int) {
	if from == to {
		return
	}
	start, end := min(from, to), max(from, to)
	e.kills.add(e.buf[start:end], from > to, isKill(e.last))
	e.buf = slices.Delete(e.buf, start, end)
	e.point = start
}

// changeCase applies to from the cursor to the end of the current or next
// word, leaving the cursor there. A nil fn means capitalize: the first
// letter of each word (counting from the cursor) goes upper, the rest lower.
func (e *Editor) changeCase(fn func(rune) rune) {
	end := forwardWord(e.buf, e.point)
	inWord := false
	for i := e.point; i < end; i++ {
		c := e.buf[i]
		if !isWordRune(c) {
			inWord = false
			continue
		}
		switch {
		case fn != nil:
			e.buf[i] = fn(c)
		case inWord:
			e.buf[i] = unicode.ToLower(c)
		default:
			e.buf[i] = unicode.ToUpper(c)
		}
		inWord = true
	}
	e.point = end
}

// transposeWords follows readline's rl_transpose_words: swap the word
// before the cursor with the one after it (or the last two words at the end
// of the line) and leave the cursor after the second.
func (e *Editor) transposeWords() Event {
	w2End := forwardWord(e.buf, e.point)
	w2Beg := backwardWord(e.buf, w2End)
	w1Beg := backwardWord(e.buf, w2Beg)
	w1End := forwardWord(e.buf, w1Beg)
	if w1Beg == w2Beg || w2Beg < w1End {
		return EventDing
	}
	out := make([]rune, 0, len(e.buf))
	out = append(out, e.buf[:w1Beg]...)
	out = append(out, e.buf[w2Beg:w2End]...)
	out = append(out, e.buf[w1End:w2Beg]...)
	out = append(out, e.buf[w1Beg:w1End]...)
	out = append(out, e.buf[w2End:]...)
	e.buf = out
	e.point = w2End
	return EventNone
}

// yankLastArg inserts the last word of the previous history entry. Pressing
// it again replaces that with the last word of the entry before, and so on.
func (e *Editor) yankLastArg() Event {
	count := 1
	if e.last == ActYankLastArg {
		count = e.lastArgCount + 1
	}
	// Like bash, the previous insertion goes first, even if there turns
	// out to be nothing older to replace it with.
	if e.last == ActYankLastArg {
		e.buf = slices.Delete(e.buf, e.yankStart, e.yankEnd)
		e.point = e.yankStart
		e.yankEnd = e.yankStart
	}
	e.lastArgCount = count
	idx := e.histPos - count
	if idx < 0 {
		return EventDing
	}
	args := SplitArgs(e.history[idx])
	if len(args) == 0 {
		return EventDing
	}
	e.yankStart = e.point
	e.insert([]rune(args[len(args)-1]))
	e.yankEnd = e.point
	return EventNone
}

// loadLine replaces the line while browsing history. Unlike bash, edits made
// to a history entry are not remembered when you move away from it.
func (e *Editor) loadLine(r []rune) {
	e.buf = slices.Clone(r)
	e.point = len(e.buf)
	e.undo = nil
}
