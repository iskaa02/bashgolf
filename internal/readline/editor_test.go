package readline

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

// cursorMark marks the cursor position in test strings.
const cursorMark = "‸"

// testCase is shared with a script that replays the same cases in real
// bash, so every expectation here is checked against actual readline.
type testCase struct {
	Name    string   `json:"name"`
	History []string `json:"history"`
	Start   string   `json:"start"`
	Keys    []string `json:"keys"` // key names; 'quoted' entries are typed text
	Want    string   `json:"want"`
	Bash    *bool    `json:"bash"` // false: our behaviour deliberately differs
}

func parseLine(t *testing.T, s string) (string, int) {
	t.Helper()
	i := strings.Index(s, cursorMark)
	if i < 0 {
		t.Fatalf("no cursor mark in %q", s)
	}
	text := strings.Replace(s, cursorMark, "", 1)
	return text, len([]rune(s[:i]))
}

func render(e *Editor) string {
	r := []rune(e.Text())
	return string(r[:e.Point()]) + cursorMark + string(r[e.Point():])
}

func TestCases(t *testing.T) {
	data, err := os.ReadFile("testdata/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []testCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			text, point := parseLine(t, tc.Start)
			e := New(text, point)
			e.SetHistory(tc.History)
			for _, k := range tc.Keys {
				if len(k) >= 2 && strings.HasPrefix(k, "'") && strings.HasSuffix(k, "'") {
					for _, r := range k[1 : len(k)-1] {
						e.Feed(Key{Name: string(r), Text: string(r)})
					}
					continue
				}
				if r := e.Feed(Key{Name: k}); r.Unbound {
					t.Fatalf("key %q is unbound", k)
				}
			}
			if got := render(e); got != tc.Want {
				t.Errorf("keys %v\n got: %q\nwant: %q", tc.Keys, got, tc.Want)
			}
		})
	}
}

func TestDeleteCharOnEmptyLineIsEOF(t *testing.T) {
	e := New("", 0)
	if r := e.Feed(Key{Name: "ctrl+d"}); r.Event != EventEOF {
		t.Errorf("event = %v, want EOF", r.Event)
	}
	e = New("x", 1)
	if r := e.Feed(Key{Name: "ctrl+d"}); r.Event != EventDing {
		t.Errorf("ctrl+d at end of non-empty line: event = %v, want ding", r.Event)
	}
}

func TestKillRingSurvivesSubmit(t *testing.T) {
	e := New("secret", 6)
	e.Feed(Key{Name: "ctrl+u"})
	e.Submit()
	e.Feed(Key{Name: "ctrl+y"})
	if e.Text() != "secret" {
		t.Errorf("text = %q, want %q", e.Text(), "secret")
	}
}

func TestSubmitAddsHistory(t *testing.T) {
	e := New("ls", 2)
	if got := e.Submit(); got != "ls" {
		t.Errorf("Submit = %q", got)
	}
	e.Insert("   ")
	e.Submit()
	if !reflect.DeepEqual(e.History(), []string{"ls"}) {
		t.Errorf("history = %q, blank lines should be skipped", e.History())
	}
}

func TestKillRingIsBounded(t *testing.T) {
	e := New("", 0)
	for range killRingMax + 5 {
		e.Insert("x ")
		e.Feed(Key{Name: "ctrl+a"}) // break the kill chain
		e.Feed(Key{Name: "ctrl+k"})
	}
	if entries, _ := e.KillRing().Entries(); len(entries) != killRingMax {
		t.Errorf("kill ring has %d entries, want %d", len(entries), killRingMax)
	}
}

func TestUnknownChordDings(t *testing.T) {
	e := New("abc", 1)
	if r := e.Feed(Key{Name: "ctrl+x"}); !r.Pending {
		t.Fatal("ctrl+x should wait for a second key")
	}
	if r := e.Feed(Key{Name: "q"}); r.Event != EventDing {
		t.Errorf("ctrl+x q: event = %v, want ding", r.Event)
	}
	if e.Pending() != "" {
		t.Error("chord should be cleared")
	}
}

func TestSplitArgs(t *testing.T) {
	tests := map[string][]string{
		`ls -la`:                     {"ls", "-la"},
		`mkdir test && cd test`:      {"mkdir", "test", "&&", "cd", "test"},
		`git commit -m "fix it now"`: {"git", "commit", "-m", `"fix it now"`},
		`echo it\'s ok`:              {"echo", `it\'s`, "ok"},
		`ps aux|grep vim`:            {"ps", "aux", "|", "grep", "vim"},
		`   `:                        nil,
	}
	for in, want := range tests {
		if got := SplitArgs(in); !reflect.DeepEqual(got, want) {
			t.Errorf("SplitArgs(%q) = %q, want %q", in, got, want)
		}
	}
}
