package readline

import (
	"encoding/json"
	"os"
	"testing"
)

// The expectations in testdata/expand.json come from real bash; regenerate
// them with tools/expand_oracle.py after adding inputs.
func TestExpandMatchesBash(t *testing.T) {
	data, err := os.ReadFile("testdata/expand.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		History []string `json:"history"`
		Line    string   `json:"line"`
		Want    string   `json:"want"`
		Error   bool     `json:"error"`
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		got, err := Expand(c.Line, c.History)
		switch {
		case c.Error && err == nil:
			t.Errorf("Expand(%q) with history %q = %q, want an error", c.Line, c.History, got)
		case !c.Error && err != nil:
			t.Errorf("Expand(%q) with history %q: %v, want %q", c.Line, c.History, err, c.Want)
		case !c.Error && got != c.Want:
			t.Errorf("Expand(%q) with history %q = %q, want %q", c.Line, c.History, got, c.Want)
		}
	}
}
