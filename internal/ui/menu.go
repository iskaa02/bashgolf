package ui

import (
	"fmt"
	"math"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/iskaa02/bashgolf/internal/game"
)

const logo = `
██████╗  █████╗ ███████╗██╗  ██╗ ██████╗  ██████╗ ██╗     ███████╗
██╔══██╗██╔══██╗██╔════╝██║  ██║██╔════╝ ██╔═══██╗██║     ██╔════╝
██████╔╝███████║███████╗███████║██║  ███╗██║   ██║██║     █████╗
██╔══██╗██╔══██║╚════██║██╔══██║██║   ██║██║   ██║██║     ██╔══╝
██████╔╝██║  ██║███████║██║  ██║╚██████╔╝╚██████╔╝███████╗██║
╚═════╝ ╚═╝  ╚═╝╚══════╝╚═╝  ╚═╝ ╚═════╝  ╚═════╝ ╚══════╝╚═╝`

// logoStops is the green palette the logo cycles through. The last stop
// leads back into the first so the gradient can scroll without a seam.
var logoStops = [][3]float64{
	{0x05, 0x6e, 0x3c}, // deep emerald
	{0x12, 0xb8, 0x6a}, // jade
	{0x4d, 0xff, 0xa0}, // mint
	{0xc6, 0xff, 0x5e}, // lime glow
	{0x4d, 0xff, 0xa0},
	{0x12, 0xb8, 0x6a},
}

const (
	logoFrames    = 48
	logoFrameTime = 70 * time.Millisecond
	logoShade     = 0.45 // brightness of the box-drawing shadow
)

type logoTick struct{ m *menu }

// logoFrameCache holds every frame of the animated logo, built on first use.
var logoFrameCache []string

func logoFrame(i int) string {
	if logoFrameCache == nil {
		for f := range logoFrames {
			logoFrameCache = append(logoFrameCache, gradientLogo(float64(f)/logoFrames))
		}
	}
	return logoFrameCache[i%logoFrames]
}

// gradientLogo paints the logo with a diagonal green gradient shifted by
// phase (0 to 1). Solid blocks get the full colour and the box-drawing
// shadow a darker shade of it.
func gradientLogo(phase float64) string {
	lines := strings.Split(strings.TrimPrefix(logo, "\n"), "\n")
	width := 0
	for _, l := range lines {
		width = max(width, len([]rune(l)))
	}
	span := float64(width + 2*len(lines))
	var b strings.Builder
	for row, l := range lines {
		for col, r := range []rune(l) {
			if r == ' ' {
				b.WriteRune(r)
				continue
			}
			c := gradientAt(float64(col+2*row)/span - phase)
			if r != '█' {
				for i := range c {
					c[i] *= logoShade
				}
			}
			hex := fmt.Sprintf("#%02x%02x%02x", int(c[0]), int(c[1]), int(c[2]))
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(hex)).Bold(true).Render(string(r)))
		}
		if row < len(lines)-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// gradientAt returns the colour at t on the looping palette; only the
// fractional part of t matters.
func gradientAt(t float64) [3]float64 {
	t -= math.Floor(t)
	seg := t * float64(len(logoStops))
	i := int(seg) % len(logoStops)
	j := (i + 1) % len(logoStops)
	f := seg - math.Floor(seg)
	f = f * f * (3 - 2*f) // smoothstep, so the bands blend softly
	var c [3]float64
	for k := range c {
		c[k] = logoStops[i][k] + (logoStops[j][k]-logoStops[i][k])*f
	}
	return c
}

type menuItem struct {
	title, desc string
	open        func() Screen
}

var (
	killRingPack = game.MustLoadPack("killring")
	caseFixPack  = game.MustLoadPack("casefix")
	historyPack  = game.MustLoadPack("history")
)

type menu struct {
	items  []menuItem
	cursor int
	width  int
	frame  int
}

func newMenu(prog *game.Progress) *menu {
	return &menu{items: []menuItem{
		{"Kill Ring Surgeon", "fix broken commands by cutting and pasting", func() Screen { return newLevelSelect(killRingPack, prog, 0) }},
		{"Case Fixer", "Caps Lock accidents and swapped letters", func() Screen { return newLevelSelect(caseFixPack, prog, 0) }},
		{"History Detective", "rerun old commands with Ctrl+R, !! and friends", func() Screen { return newLevelSelect(historyPack, prog, 0) }},
		{"Cursor Dash", "race the cursor to targets: free run or levels", func() Screen { return newDash(prog) }},
		{"Line Rush", "fix broken commands against the clock: free run or levels", func() Screen { return newRush(prog) }},
		{"Key Check", "see which shortcuts your terminal sends", func() Screen { return newKeyCheck() }},
		{"Sandbox", "a fake shell line to practise every shortcut", func() Screen { return newSandbox() }},
	}}
}

// Init starts the logo animation.
func (m *menu) Init() tea.Cmd { return m.tick() }

func (m *menu) tick() tea.Cmd {
	return tea.Tick(logoFrameTime, func(time.Time) tea.Msg { return logoTick{m} })
}

func (m *menu) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case logoTick:
		// Ticks from a menu that has since been replaced are dropped, so
		// only one animation runs at a time.
		if msg.m == m {
			m.frame++
			return m, m.tick()
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k", "ctrl+p":
			m.cursor = (m.cursor + len(m.items) - 1) % len(m.items)
		case "down", "j", "ctrl+n", "tab":
			m.cursor = (m.cursor + 1) % len(m.items)
		case "enter", " ":
			return m, switchTo(m.items[m.cursor].open())
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *menu) View() string {
	var b strings.Builder
	b.WriteString("\n" + logoFrame(m.frame) + "\n")
	b.WriteString(dimStyle.Render("   learn your shell's keyboard shortcuts") + "\n\n")
	for i, it := range m.items {
		if i == m.cursor {
			b.WriteString(keyStyle.Render(" ▸ "+it.title) + "  " + textStyle.Render(it.desc) + "\n")
		} else {
			b.WriteString(dimStyle.Render("   "+it.title+"  "+it.desc) + "\n")
		}
	}
	b.WriteString("\n" + helpStyle.Render("   ↑/↓ choose · enter start · q quit"))
	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}
