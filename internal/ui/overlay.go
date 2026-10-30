package ui

import (
	"errors"
	"strings"

	"github.com/MSarandev/sugarlime/internal/schema"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type overlayKind int

const (
	ovText overlayKind = iota
	ovChoice
	ovJSON
)

// valueCustom is the sentinel a choice uses to ask for free text instead.
const valueCustom = "\x00custom"

// errKeepOverlay means the action replaced the overlay; don't close it.
var errKeepOverlay = errors.New("keep overlay")

type choice struct {
	label  string
	desc   string
	value  string
	danger bool
}

// overlay is the modal in front of the browser.
type overlay struct {
	kind  overlayKind
	title string
	hint  string

	input textinput.Model
	area  textarea.Model

	choices []choice
	cursor  int

	// results lets a text overlay offer matches from the reference.
	results  []schema.Field
	onResult func(*App, schema.Field) (string, error)

	completions []string
	errMsg      string

	validate    func(string) error
	submit      func(*App, string) (string, error)
	live        func(*App, string)
	emptyUnsets bool
}

func textOverlay(title, hint, value string, validate func(string) error, submit func(*App, string) (string, error)) *overlay {
	in := textinput.New()
	in.SetValue(value)
	in.CursorEnd()
	in.Focus()
	in.Prompt = "› "
	in.CharLimit = 2048
	return &overlay{kind: ovText, title: title, hint: hint, input: in, validate: validate, submit: submit}
}

func choiceOverlay(title, hint string, choices []choice, selected int, submit func(*App, string) (string, error)) *overlay {
	return &overlay{
		kind:    ovChoice,
		title:   title,
		hint:    hint,
		choices: choices,
		cursor:  clamp(selected, 0, max(0, len(choices)-1)),
		submit:  submit,
	}
}

func confirmOverlay(title, hint string, choices []choice, submit func(*App, string) (string, error)) *overlay {
	return choiceOverlay(title, hint, choices, 0, submit)
}

func jsonOverlay(title, hint, value string, submit func(*App, string) (string, error)) *overlay {
	ta := textarea.New()
	ta.SetValue(value)
	ta.Focus()
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	ta.SetWidth(60)
	ta.SetHeight(12)
	return &overlay{kind: ovJSON, title: title, hint: hint, area: ta, submit: submit}
}

// overlayKey routes a key press to the open overlay.
func (a *App) overlayKey(msg tea.KeyMsg) tea.Cmd {
	ov := a.ov
	key := msg.String()

	switch ov.kind {
	case ovChoice:
		switch key {
		case "esc", "q", "ctrl+c":
			a.ov = nil
		case "up", "k":
			ov.cursor = clamp(ov.cursor-1, 0, len(ov.choices)-1)
		case "down", "j":
			ov.cursor = clamp(ov.cursor+1, 0, len(ov.choices)-1)
		case "home":
			ov.cursor = 0
		case "end":
			ov.cursor = len(ov.choices) - 1
		case "enter", "right", "l", " ":
			if ov.cursor < len(ov.choices) {
				a.commit(ov.choices[ov.cursor].value)
			}
		default:
			// A digit picks a choice directly.
			if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
				if i := int(key[0] - '1'); i < len(ov.choices) {
					ov.cursor = i
					a.commit(ov.choices[i].value)
				}
			}
		}
		return nil

	case ovJSON:
		switch key {
		case "esc":
			a.ov = nil
			return nil
		case "ctrl+c":
			a.ov = nil
			return nil
		case "ctrl+s", "ctrl+d":
			a.commit(ov.area.Value())
			return nil
		}
		var cmd tea.Cmd
		ov.area, cmd = ov.area.Update(msg)
		return cmd

	default: // ovText
		switch key {
		case "esc", "ctrl+c":
			a.ov = nil
			return nil
		case "enter":
			a.commit(ov.input.Value())
			return nil
		case "down", "ctrl+n":
			if len(ov.results) > 0 {
				ov.cursor = clamp(ov.cursor+1, 0, len(ov.results)-1)
				return nil
			}
		case "up", "ctrl+p":
			if len(ov.results) > 0 {
				ov.cursor = clamp(ov.cursor-1, 0, len(ov.results)-1)
				return nil
			}
		case "tab":
			if c := ov.completion(); c != "" {
				ov.input.SetValue(c)
				ov.input.CursorEnd()
				return nil
			}
		}
		before := ov.input.Value()
		var cmd tea.Cmd
		ov.input, cmd = ov.input.Update(msg)
		if ov.input.Value() != before {
			ov.errMsg = ""
			ov.cursor = 0
			if ov.live != nil {
				ov.live(a, ov.input.Value())
			}
		}
		return cmd
	}
}

// completion returns the one candidate matching what was typed, if only one does.
func (ov *overlay) completion() string {
	typed := strings.TrimSpace(ov.input.Value())
	if typed == "" || len(ov.completions) == 0 {
		return ""
	}
	var hit string
	for _, c := range ov.completions {
		if strings.HasPrefix(strings.ToLower(c), strings.ToLower(typed)) {
			if hit != "" {
				return ""
			}
			hit = c
		}
	}
	return hit
}

// commit runs the overlay's action, keeping it open on an error.
func (a *App) commit(value string) {
	ov := a.ov

	if ov.kind == ovText && len(ov.results) > 0 && ov.onResult != nil {
		f := ov.results[clamp(ov.cursor, 0, len(ov.results)-1)]
		a.ov = nil
		if msg, err := ov.onResult(a, f); err == nil && msg != "" {
			a.setStatus(msg, statusInfo)
		}
		return
	}

	if ov.validate != nil && !(ov.emptyUnsets && strings.TrimSpace(value) == "") {
		if err := ov.validate(value); err != nil {
			ov.errMsg = err.Error()
			return
		}
	}

	msg, err := ov.submit(a, value)
	switch {
	case errors.Is(err, errKeepOverlay):
		return
	case err != nil:
		ov.errMsg = err.Error()
		return
	}
	a.ov = nil
	if msg != "" {
		a.setStatus(msg, statusGood)
	}
}
