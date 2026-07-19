package tui

import (
	"errors"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/mateidumitrascu/typepractice/internal/client"
	"github.com/mateidumitrascu/typepractice/internal/store"
	"github.com/mateidumitrascu/typepractice/internal/theme"
	"github.com/mateidumitrascu/typepractice/internal/typing"
)

type screen int

const (
	screenSetup screen = iota
	screenLoading
	screenLogin
	screenMenu
	screenLetterPick
	screenTyping
	screenStats
	screenThemes
)

type saveState int

const (
	saveNone saveState = iota
	saveInFlight
	saveDone
	saveFailed
)

var menuItems = []struct{ label, desc, mode string }{
	{"speed test", "a 30–45 word set · wpm and accuracy recorded", "speed"},
	{"free practice", "endless typing · nothing recorded", "free"},
	{"letter focus", "pick a letter · sets built around it", "letter"},
	{"stats", "progress over time · personal bests", ""},
	{"themes", "change how this looks", ""},
	{"quit", "", ""},
}

type App struct {
	cfg    Config
	client *client.Client
	themes []theme.Theme
	s      Styles

	width, height int
	screen        screen
	spin          spinner.Model

	setupInput textinput.Model
	setupErr   string
	busy       bool

	userInput textinput.Model
	passInput textinput.Model
	loginErr  string

	menuIdx int
	menuErr string

	mode      string
	letter    rune
	eng       *typing.Engine
	result    *typing.Result
	save      saveState
	saveErr   string
	freeWords int

	stats *store.Stats

	themeIdx      int
	themeSavedIdx int
	themeNote     string
}

func NewApp(cfg Config) *App {
	a := &App{cfg: cfg, s: NewStyles(theme.Default())}

	a.spin = spinner.New()
	a.spin.Spinner = spinner.MiniDot

	a.setupInput = textinput.New()
	a.setupInput.Placeholder = "http://localhost:8080"
	a.setupInput.Prompt = ""
	a.setupInput.CharLimit = 200

	a.userInput = textinput.New()
	a.userInput.Placeholder = "username"
	a.userInput.Prompt = ""
	a.passInput = textinput.New()
	a.passInput.Placeholder = "password"
	a.passInput.Prompt = ""
	a.passInput.EchoMode = textinput.EchoPassword
	a.passInput.EchoCharacter = '•'

	a.applyStyles()
	return a
}

// applyStyles re-derives widget styles after a theme change.
func (a *App) applyStyles() {
	a.spin.Style = a.s.Accent
	for _, ti := range []*textinput.Model{&a.setupInput, &a.userInput, &a.passInput} {
		ti.TextStyle = a.s.Text
		ti.PlaceholderStyle = a.s.Subtext
		ti.Cursor.Style = a.s.Accent
	}
}

func (a *App) Init() tea.Cmd {
	if a.cfg.Server == "" {
		a.screen = screenSetup
		a.setupInput.SetValue("http://localhost:8080")
		return a.setupInput.Focus()
	}
	a.client = client.New(a.cfg.Server, a.cfg.Token)
	a.screen = screenLoading
	return tea.Batch(a.spin.Tick, a.fetchThemes())
}

// ---- messages ----

type themesMsg struct {
	themes []theme.Theme
	def    string
	err    error
}
type authMsg struct {
	theme string
	err   error
}
type loginDoneMsg struct {
	token   string
	expires time.Time
	err     error
}
type wordsMsg struct {
	words []string
	err   error
}
type savedMsg struct{ err error }
type statsMsg struct {
	stats store.Stats
	err   error
}
type themePutMsg struct{ err error }
type tickMsg struct{}

// ---- commands ----

func (a *App) fetchThemes() tea.Cmd {
	c := a.client
	return func() tea.Msg {
		themes, def, err := c.Themes()
		return themesMsg{themes, def, err}
	}
}

func (a *App) checkAuth() tea.Cmd {
	c := a.client
	return func() tea.Msg {
		t, err := c.Theme()
		return authMsg{t, err}
	}
}

func (a *App) doLogin(user, pass string) tea.Cmd {
	c := a.client
	return func() tea.Msg {
		token, expires, err := c.Login(user, pass)
		return loginDoneMsg{token, expires, err}
	}
}

func (a *App) fetchWords() tea.Cmd {
	c, mode, letter := a.client, a.mode, a.letter
	return func() tea.Msg {
		words, err := c.Words(mode, letter)
		return wordsMsg{words, err}
	}
}

func (a *App) submitResult(r typing.Result) tea.Cmd {
	c := a.client
	res := store.Result{
		Mode:       a.mode,
		WordCount:  r.WordCount,
		DurationMs: r.Duration.Milliseconds(),
		CharsTyped: r.CharsTyped,
		Errors:     r.UncorrectedErrors,
		Accuracy:   r.Accuracy,
	}
	if a.mode == "letter" {
		res.Letter = string(a.letter)
	}
	return func() tea.Msg {
		return savedMsg{c.SubmitResult(res)}
	}
}

func (a *App) fetchStats() tea.Cmd {
	c := a.client
	return func() tea.Msg {
		st, err := c.Stats()
		return statsMsg{st, err}
	}
}

func (a *App) putTheme(name string) tea.Cmd {
	c := a.client
	return func() tea.Msg {
		return themePutMsg{c.SetTheme(name)}
	}
}

func tick() tea.Cmd {
	return tea.Tick(150*time.Millisecond, func(time.Time) tea.Msg { return tickMsg{} })
}

// ---- update ----

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		return a, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return a, tea.Quit
		}
		return a.updateKey(msg)

	case spinner.TickMsg:
		if a.busy || a.screen == screenLoading {
			var cmd tea.Cmd
			a.spin, cmd = a.spin.Update(msg)
			return a, cmd
		}
		return a, nil

	case tickMsg:
		if a.screen == screenTyping && a.eng != nil && a.eng.Started() && !a.eng.Done() {
			return a, tick()
		}
		return a, nil

	case themesMsg:
		if msg.err != nil {
			a.screen = screenSetup
			a.setupErr = "cannot reach server: " + msg.err.Error()
			if a.setupInput.Value() == "" {
				a.setupInput.SetValue(a.cfg.Server)
			}
			a.busy = false
			return a, a.setupInput.Focus()
		}
		a.themes = msg.themes
		if a.cfg.Server != a.setupInput.Value() && a.setupInput.Value() != "" {
			a.cfg.Server = a.setupInput.Value()
			SaveConfig(a.cfg)
		}
		a.busy = false
		a.screen = screenLoading
		return a, tea.Batch(a.spin.Tick, a.checkAuth())

	case authMsg:
		if errors.Is(msg.err, client.ErrUnauthorized) {
			a.screen = screenLogin
			a.userInput.Focus()
			a.passInput.Blur()
			return a, nil
		}
		if msg.err != nil {
			a.screen = screenSetup
			a.setupErr = msg.err.Error()
			a.setupInput.SetValue(a.cfg.Server)
			return a, a.setupInput.Focus()
		}
		a.applyTheme(msg.theme)
		a.screen = screenMenu
		return a, nil

	case loginDoneMsg:
		a.busy = false
		if msg.err != nil {
			a.loginErr = msg.err.Error()
			a.passInput.SetValue("")
			return a, nil
		}
		a.cfg.Token = msg.token
		a.cfg.ExpiresAt = msg.expires
		SaveConfig(a.cfg)
		a.loginErr = ""
		a.screen = screenLoading
		return a, tea.Batch(a.spin.Tick, a.checkAuth())

	case wordsMsg:
		a.busy = false
		if msg.err != nil {
			return a.apiError(msg.err, "couldn't fetch words: ")
		}
		if a.mode == "free" && a.eng != nil {
			a.freeWords += a.eng.Index()
		}
		a.eng = typing.New(msg.words)
		a.result = nil
		a.save = saveNone
		a.screen = screenTyping
		return a, nil

	case savedMsg:
		if msg.err != nil {
			a.save = saveFailed
			a.saveErr = msg.err.Error()
		} else {
			a.save = saveDone
		}
		return a, nil

	case statsMsg:
		a.busy = false
		if msg.err != nil {
			return a.apiError(msg.err, "couldn't fetch stats: ")
		}
		a.stats = &msg.stats
		a.screen = screenStats
		return a, nil

	case themePutMsg:
		if msg.err != nil {
			a.themeNote = "save failed: " + msg.err.Error()
		} else {
			a.themeSavedIdx = a.themeIdx
			a.themeNote = "saved"
		}
		return a, nil
	}
	return a, nil
}

// apiError routes 401s to the login screen, everything else to a menu banner.
func (a *App) apiError(err error, prefix string) (tea.Model, tea.Cmd) {
	if errors.Is(err, client.ErrUnauthorized) {
		a.screen = screenLogin
		a.loginErr = "session expired — sign in again"
		a.userInput.Focus()
		a.passInput.Blur()
		return a, nil
	}
	a.menuErr = prefix + err.Error()
	a.screen = screenMenu
	return a, nil
}

func (a *App) applyTheme(name string) {
	for i, t := range a.themes {
		if t.Name == name {
			a.s = NewStyles(t)
			a.themeIdx, a.themeSavedIdx = i, i
			a.applyStyles()
			return
		}
	}
}

func (a *App) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch a.screen {
	case screenSetup:
		return a.updateSetup(msg)
	case screenLogin:
		return a.updateLogin(msg)
	case screenMenu:
		return a.updateMenu(msg)
	case screenLetterPick:
		return a.updateLetterPick(msg)
	case screenTyping:
		return a.updateTyping(msg)
	case screenStats:
		if msg.String() == "esc" || msg.String() == "q" {
			a.screen = screenMenu
		}
		return a, nil
	case screenThemes:
		return a.updateThemes(msg)
	}
	return a, nil
}

func (a *App) updateSetup(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "enter" && !a.busy {
		url := a.setupInput.Value()
		if url == "" {
			return a, nil
		}
		a.busy = true
		a.setupErr = ""
		a.client = client.New(url, a.cfg.Token)
		return a, tea.Batch(a.spin.Tick, a.fetchThemes())
	}
	var cmd tea.Cmd
	a.setupInput, cmd = a.setupInput.Update(msg)
	return a, cmd
}

func (a *App) updateLogin(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab", "shift+tab", "up", "down":
		if a.userInput.Focused() {
			a.userInput.Blur()
			return a, a.passInput.Focus()
		}
		a.passInput.Blur()
		return a, a.userInput.Focus()
	case "enter":
		if a.userInput.Focused() {
			a.userInput.Blur()
			return a, a.passInput.Focus()
		}
		if a.busy || a.userInput.Value() == "" || a.passInput.Value() == "" {
			return a, nil
		}
		a.busy = true
		a.loginErr = ""
		return a, tea.Batch(a.spin.Tick, a.doLogin(a.userInput.Value(), a.passInput.Value()))
	}
	var cmd tea.Cmd
	if a.userInput.Focused() {
		a.userInput, cmd = a.userInput.Update(msg)
	} else {
		a.passInput, cmd = a.passInput.Update(msg)
	}
	return a, cmd
}

func (a *App) updateMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		a.menuIdx = (a.menuIdx + len(menuItems) - 1) % len(menuItems)
	case "down", "j":
		a.menuIdx = (a.menuIdx + 1) % len(menuItems)
	case "q":
		return a, tea.Quit
	case "enter":
		a.menuErr = ""
		switch menuItems[a.menuIdx].label {
		case "speed test", "free practice":
			a.mode = menuItems[a.menuIdx].mode
			a.letter = 0
			a.freeWords = 0
			a.eng = nil
			a.busy = true
			return a, tea.Batch(a.spin.Tick, a.fetchWords())
		case "letter focus":
			a.mode = "letter"
			a.screen = screenLetterPick
		case "stats":
			a.busy = true
			return a, tea.Batch(a.spin.Tick, a.fetchStats())
		case "themes":
			a.themeNote = ""
			a.screen = screenThemes
		case "quit":
			return a, tea.Quit
		}
	}
	return a, nil
}

func (a *App) updateLetterPick(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := msg.String()
	if s == "esc" {
		a.screen = screenMenu
		return a, nil
	}
	if len(s) == 1 && s[0] >= 'a' && s[0] <= 'z' {
		a.letter = rune(s[0])
		a.eng = nil
		a.busy = true
		return a, tea.Batch(a.spin.Tick, a.fetchWords())
	}
	return a, nil
}

func (a *App) updateTyping(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := msg.String()

	if a.result != nil {
		switch s {
		case "enter", "tab":
			a.busy = true
			return a, tea.Batch(a.spin.Tick, a.fetchWords())
		case "esc":
			a.screen = screenMenu
		}
		return a, nil
	}

	switch s {
	case "esc":
		a.eng = nil
		a.screen = screenMenu
		return a, nil
	case "tab":
		a.busy = true
		return a, tea.Batch(a.spin.Tick, a.fetchWords())
	case "backspace":
		if a.eng != nil {
			a.eng.Backspace()
		}
		return a, nil
	}

	if a.eng == nil || a.busy {
		return a, nil
	}
	var cmd tea.Cmd
	started := a.eng.Started()
	if s == " " {
		a.eng.Space()
	} else if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 {
		a.eng.Type(msg.Runes[0])
	} else {
		return a, nil
	}
	if !started && a.eng.Started() {
		cmd = tick()
	}
	if a.eng.Done() {
		if a.mode == "free" {
			a.busy = true
			return a, tea.Batch(a.spin.Tick, a.fetchWords())
		}
		r := a.eng.Result()
		a.result = &r
		a.save = saveInFlight
		return a, a.submitResult(r)
	}
	return a, cmd
}

func (a *App) updateThemes(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		a.themeIdx = (a.themeIdx + len(a.themes) - 1) % len(a.themes)
		a.previewTheme()
	case "down", "j":
		a.themeIdx = (a.themeIdx + 1) % len(a.themes)
		a.previewTheme()
	case "enter":
		a.themeNote = "saving…"
		return a, a.putTheme(a.themes[a.themeIdx].Name)
	case "esc":
		a.themeIdx = a.themeSavedIdx
		a.previewTheme()
		a.screen = screenMenu
	}
	return a, nil
}

func (a *App) previewTheme() {
	if a.themeIdx < len(a.themes) {
		a.s = NewStyles(a.themes[a.themeIdx])
		a.applyStyles()
		a.themeNote = ""
	}
}
