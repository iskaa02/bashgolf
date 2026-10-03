package readline

import "slices"

// killRingMax matches readline's default kill ring size.
const killRingMax = 10

// KillRing stores cut text. Entries are oldest first; idx is the entry the
// next yank will insert, which yank-pop rotates backwards through.
type KillRing struct {
	entries [][]rune
	idx     int
}

// add stores a kill. When merge is set (the previous command was also a
// kill) the text joins the newest entry instead, before it for backward
// kills and after it for forward ones, which is how readline lets several
// Ctrl+W presses be yanked back as one piece.
func (k *KillRing) add(text []rune, backward, merge bool) {
	if merge && len(k.entries) > 0 {
		last := len(k.entries) - 1
		if backward {
			k.entries[last] = append(slices.Clone(text), k.entries[last]...)
		} else {
			k.entries[last] = append(k.entries[last], text...)
		}
	} else {
		k.entries = append(k.entries, slices.Clone(text))
		if len(k.entries) > killRingMax {
			k.entries = k.entries[1:]
		}
	}
	k.idx = len(k.entries) - 1
}

func (k *KillRing) current() []rune {
	if len(k.entries) == 0 {
		return nil
	}
	return k.entries[k.idx]
}

func (k *KillRing) rotate() {
	k.idx--
	if k.idx < 0 {
		k.idx = len(k.entries) - 1
	}
}

// Entries returns the ring oldest first, and the index the next yank uses.
func (k *KillRing) Entries() ([]string, int) {
	out := make([]string, len(k.entries))
	for i, e := range k.entries {
		out[i] = string(e)
	}
	return out, k.idx
}
