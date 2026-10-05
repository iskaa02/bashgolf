package readline

import (
	"fmt"
	"slices"
)

// SearchState describes an incremental history search in progress, for the
// "(reverse-i-search)`query': line" prompt.
type SearchState struct {
	Active  bool
	Query   string
	Forward bool
	Failed  bool
}

// isearch is Ctrl+R / Ctrl+S. Behaviour follows bash (checked with
// tools/readline_oracle.py): reverse search finds the last match in a line
// and leaves the cursor at its start; repeating Ctrl+R looks further back,
// first in the same line; the line being typed is searched too; any key the
// search doesn't use ends it and then does its normal job.
type isearch struct {
	active  bool
	query   []rune
	forward bool
	failed  bool
	matched bool // something has matched in this search

	lines   []string // history plus the line being edited, at search start
	lineIdx int      // line of the current match (or the start line)
	pos     int      // match start (or the start cursor)

	origBuf   []rune
	origPoint int
	origHist  int
}

// Search reports the incremental search state.
func (e *Editor) Search() SearchState {
	s := &e.search
	return SearchState{Active: s.active, Query: string(s.query), Forward: s.forward, Failed: s.failed}
}

func (e *Editor) startSearch(forward bool) {
	lines := slices.Clone(e.history)
	if e.histPos == len(e.history) {
		lines = append(lines, string(e.buf))
	} else {
		lines = append(lines, string(e.saved))
		lines[e.histPos] = string(e.buf)
	}
	e.search = isearch{
		active: true, forward: forward,
		lines: lines, lineIdx: e.histPos, pos: e.point,
		origBuf: slices.Clone(e.buf), origPoint: e.point, origHist: e.histPos,
	}
}

// feedSearch handles a key while searching. done=false means the search
// ended without using the key, which the caller must then process.
func (e *Editor) feedSearch(k Key) (res Result, done bool) {
	s := &e.search
	switch {
	case k.Text != "":
		s.query = append(s.query, []rune(k.Text)...)
		e.searchFind(true)
	case k.Name == "backspace" || k.Name == "ctrl+h":
		if len(s.query) > 0 {
			s.query = s.query[:len(s.query)-1]
		}
		if len(s.query) > 0 {
			e.searchFind(true)
		} else {
			s.failed = false
		}
	case k.Name == "ctrl+r" || k.Name == "ctrl+s":
		forward := k.Name == "ctrl+s"
		switch {
		case len(s.query) == 0:
			s.forward = forward
			if len(e.lastSearch) == 0 {
				return Result{Event: EventDing}, true
			}
			s.query = slices.Clone(e.lastSearch)
			e.searchFind(true)
		case forward != s.forward:
			s.forward = forward
			e.searchFind(true)
		default:
			e.searchFind(false)
		}
	case k.Name == "ctrl+g":
		e.buf, e.point, e.histPos = s.origBuf, s.origPoint, s.origHist
		*s = isearch{}
		e.last = ActNone
		return Result{Event: EventDing}, true
	case k.Name == "enter" || k.Name == "ctrl+j":
		e.endSearch()
		return Result{Action: ActAcceptLine, Event: EventAccept}, true
	default:
		e.endSearch()
		return Result{}, false
	}
	if s.failed {
		return Result{Event: EventDing}, true
	}
	return Result{}, true
}

// searchFind looks for the query from the current match: inclusive=true
// may stay where it is (after typing), false moves on (Ctrl+R again).
func (e *Editor) searchFind(inclusive bool) {
	s := &e.search
	q := s.query
	idx, pos := s.lineIdx, s.pos
	prevLine := ""
	if s.matched {
		prevLine = s.lines[s.lineIdx]
	}
	found := func(i, p int) {
		s.lineIdx, s.pos, s.matched, s.failed = i, p, true, false
		e.buf = []rune(s.lines[i])
		e.point = p
	}
	at := func(line []rune, i int) bool {
		return i >= 0 && i+len(q) <= len(line) && slices.Equal(line[i:i+len(q)], q)
	}

	if !s.forward {
		if !inclusive {
			pos--
		}
		for first := true; idx >= 0; idx, first = idx-1, false {
			line := []rune(s.lines[idx])
			if !first && s.matched && s.lines[idx] == prevLine {
				continue // bash skips lines identical to the last match
			}
			start := len(line) - len(q)
			if first {
				start = min(pos, start)
			}
			for i := start; i >= 0; i-- {
				if at(line, i) {
					found(idx, i)
					return
				}
			}
		}
	} else {
		if !inclusive {
			pos++
		}
		for first := true; idx < len(s.lines); idx, first = idx+1, false {
			line := []rune(s.lines[idx])
			if !first && s.matched && s.lines[idx] == prevLine {
				continue
			}
			start := 0
			if first {
				start = max(pos, 0)
			}
			for i := start; i+len(q) <= len(line); i++ {
				if at(line, i) {
					found(idx, i)
					return
				}
			}
		}
	}
	s.failed = true
}

// endSearch keeps the matched line, as if you had navigated to it.
func (e *Editor) endSearch() {
	s := &e.search
	if len(s.query) > 0 {
		e.lastSearch = slices.Clone(s.query)
	}
	if s.matched && s.lineIdx != s.origHist {
		if s.origHist == len(e.history) {
			e.saved = s.origBuf
		}
		if s.lineIdx == len(e.history) {
			e.histPos = len(e.history)
		} else {
			e.histPos = s.lineIdx
		}
		e.undo = nil
	}
	*s = isearch{}
	e.last = ActNone
}

func (s *isearch) fingerprint() string {
	if !s.active {
		return ""
	}
	return fmt.Sprintf("\x00search%v,%v,%d,%d,%s", s.forward, s.failed, s.lineIdx, s.pos, string(s.query))
}
