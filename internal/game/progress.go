package game

// Progress records the best score per level. It only lives in memory for
// now; saving it to disk comes with the progress milestone.
type Progress struct {
	best   map[string]int // "pack/level" -> fewest keys
	scores map[string]int // arcade mode -> high score
}

func NewProgress() *Progress {
	return &Progress{best: map[string]int{}, scores: map[string]int{}}
}

// HighScore returns the best score for an arcade mode like "dash".
func (p *Progress) HighScore(mode string) int { return p.scores[mode] }

// RecordScore stores a score and reports whether it is a new high score.
func (p *Progress) RecordScore(mode string, score int) bool {
	if score <= p.scores[mode] {
		return false
	}
	p.scores[mode] = score
	return true
}

func progressKey(pack *Pack, i int) string { return pack.ID + "/" + pack.Levels[i].ID }

// Best returns the fewest keys used to finish level i, if it was finished.
func (p *Progress) Best(pack *Pack, i int) (int, bool) {
	n, ok := p.best[progressKey(pack, i)]
	return n, ok
}

// Record stores a finish and reports whether it beat the previous best.
func (p *Progress) Record(pack *Pack, i int, keys int) (improved bool) {
	k := progressKey(pack, i)
	if old, ok := p.best[k]; ok && old <= keys {
		return false
	}
	p.best[k] = keys
	return true
}

// Unlocked reports whether level i can be played: the first level always,
// later ones once the level before is finished.
func (p *Progress) Unlocked(pack *Pack, i int) bool {
	if i == 0 {
		return true
	}
	_, ok := p.Best(pack, i-1)
	return ok
}
