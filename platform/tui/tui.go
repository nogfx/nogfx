package tui

import (
	"context"
	"fmt"
	"sync"

	"github.com/gdamore/tcell/v2"

	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/lib/navigation"
	"github.com/nogfx/nogfx/ui"
)

// TUI orchestrates different panes to make up the primary user interface.
// It satisfies ui.UI: its Run method emits user-driven events onto the
// provided channel, and Apply consumes commands the engine dispatches to it.
type TUI struct {
	screen tcell.Screen

	layout *Layout

	cacheMutex sync.Mutex
	panesCache map[string]Rows

	input     *Input
	cursorpos []int

	output *Output

	// events is the channel Run pushes user events onto. Set by Run; the
	// input handlers use it (via emitEvent) to surface user input.
	events chan<- app.Event

	// drawCh is used by Apply (running on the engine's goroutine) to
	// request a redraw on the UI's own goroutine. The TUI's Run loop
	// reads from drawCh and calls Draw. Buffered to length 1 so
	// back-to-back calls coalesce into a single redraw.
	drawCh chan struct{}

	// vitalsOrder preserves the order in which auxiliary vitals were added,
	// so the vitals pane renders them in a stable order.
	vitalsMu     sync.Mutex
	health       *vital
	mana         *vital
	vitalsOrder  []string
	vitalsByName map[string]*vital

	charName  string
	charTitle string

	target *ui.Target
	room   *navigation.Room

	running bool
}

type vital struct {
	Value int
	Max   int
}

// NewTUI creates a new TUI.
func NewTUI(screen tcell.Screen) *TUI {
	tui := &TUI{
		screen: screen,

		panesCache: map[string]Rows{},

		input:  &Input{},
		output: &Output{},

		drawCh:       make(chan struct{}, 1),
		vitalsByName: map[string]*vital{},
	}
	tui.layout = &Layout{tui}

	screen.SetStyle(tcell.Style{})
	screen.SetCursorStyle(tcell.CursorStyleBlinkingBlock)

	return tui
}

func (tui *TUI) setCache(name string, rows Rows) {
	tui.cacheMutex.Lock()
	defer tui.cacheMutex.Unlock()

	if rows == nil {
		delete(tui.panesCache, name)
		return
	}

	tui.panesCache[name] = rows
}

func (tui *TUI) clearCache() {
	tui.cacheMutex.Lock()
	defer tui.cacheMutex.Unlock()

	tui.panesCache = map[string]Rows{}
}

func (tui *TUI) getCache(name string) (Rows, bool) {
	tui.cacheMutex.Lock()
	defer tui.cacheMutex.Unlock()

	rows, ok := tui.panesCache[name]

	return rows, ok
}

// Run starts the user interface and pushes user-driven events onto events.
// Run blocks until ctx is cancelled (typically by the user pressing
// Ctrl+D) and satisfies ui.UI.
func (tui *TUI) Run(pctx context.Context, events chan<- app.Event) error {
	ctx, cancel := context.WithCancel(pctx)
	defer cancel()

	tui.events = events

	tui.running = true
	defer func() { tui.running = false }()

	if err := tui.screen.Init(); err != nil {
		return fmt.Errorf("failed initializing screen: %w", err)
	}
	defer tui.screen.Fini()

	go tui.pollEvents(ctx, cancel)

	tui.Draw()

	for {
		select {
		case <-ctx.Done():
			return nil

		case <-tui.drawCh:
			tui.Draw()
		}
	}
}

// pollEvents reads tcell events and translates them into TUI state changes
// or ui.Input events on the events channel.
func (tui *TUI) pollEvents(ctx context.Context, cancel func()) {
	numpad := false

	for {
		event := tui.screen.PollEvent()
		if event == nil {
			return
		}

		switch ev := event.(type) {
		case *tcell.EventResize:
			tui.clearCache()
			tui.Draw()
			tui.screen.Sync()
			tui.emitEvent(ui.Resize{})

		case *tcell.EventKey:
			if isNumpad(ev) {
				numpad = true
				continue
			} else if numpad {
				numpad = false
				ev = makeNumpad(ev)
			}

			if ev.Key() == tcell.KeyCtrlD {
				cancel()
				return
			}

			if ok := tui.HandleEvent(ev); ok {
				tui.Draw()
			}
		}

		if err := ctx.Err(); err != nil {
			return
		}
	}
}

// emitEvent pushes an event onto the events channel if Run has wired one up.
func (tui *TUI) emitEvent(ev app.Event) {
	if tui.events == nil {
		return
	}
	tui.events <- ev
}

// requestDraw asks the Run loop to redraw the screen. Safe to call from any
// goroutine.
func (tui *TUI) requestDraw() {
	select {
	case tui.drawCh <- struct{}{}:
	default:
	}
}

// Apply executes a single command against the UI. Commands not targeting
// the UI return app.ErrCommandNotApplicable.
func (tui *TUI) Apply(cmd app.Command) error {
	switch c := cmd.(type) {
	case ui.PrintLine:
		tui.output.Append(c.Text)
		tui.setCache(paneOutput, nil)
		tui.requestDraw()

	case ui.SetHealth:
		tui.vitalsMu.Lock()
		tui.health = &vital{Value: c.Value, Max: c.Max}
		tui.vitalsMu.Unlock()
		tui.setCache(paneVitals, nil)
		tui.requestDraw()

	case ui.SetMana:
		tui.vitalsMu.Lock()
		tui.mana = &vital{Value: c.Value, Max: c.Max}
		tui.vitalsMu.Unlock()
		tui.setCache(paneVitals, nil)
		tui.requestDraw()

	case ui.AddVital:
		tui.vitalsMu.Lock()
		if _, ok := tui.vitalsByName[c.Name]; !ok {
			tui.vitalsOrder = append(tui.vitalsOrder, c.Name)
		}
		tui.vitalsByName[c.Name] = &vital{Value: c.Value, Max: c.Max}
		tui.vitalsMu.Unlock()
		tui.setCache(paneVitals, nil)
		tui.requestDraw()

	case ui.SetVital:
		tui.vitalsMu.Lock()
		v, ok := tui.vitalsByName[c.Name]
		if !ok {
			tui.vitalsOrder = append(tui.vitalsOrder, c.Name)
			v = &vital{}
			tui.vitalsByName[c.Name] = v
		}
		v.Value = c.Value
		v.Max = c.Max
		tui.vitalsMu.Unlock()
		tui.setCache(paneVitals, nil)
		tui.requestDraw()

	case ui.RemoveVital:
		tui.vitalsMu.Lock()
		delete(tui.vitalsByName, c.Name)
		for i, n := range tui.vitalsOrder {
			if n == c.Name {
				tui.vitalsOrder = append(tui.vitalsOrder[:i], tui.vitalsOrder[i+1:]...)
				break
			}
		}
		tui.vitalsMu.Unlock()
		tui.setCache(paneVitals, nil)
		tui.requestDraw()

	case ui.SetCharacter:
		tui.charName = c.Name
		tui.charTitle = c.Title
		tui.requestDraw()

	case ui.SetTarget:
		tui.target = c.Target
		tui.setCache(paneTarget, nil)
		tui.requestDraw()

	case ui.SetRoom:
		tui.room = c.Room
		tui.setCache(paneMap, nil)
		tui.requestDraw()

	case ui.MaskInput:
		tui.input.masked = true
		tui.setCache(paneInput, nil)
		tui.requestDraw()

	case ui.UnmaskInput:
		tui.input.masked = false
		tui.setCache(paneInput, nil)
		tui.requestDraw()

	default:
		return app.ErrCommandNotApplicable
	}
	return nil
}

// Draw updates the terminal and prints the contents of the panes.
func (tui *TUI) Draw() {
	if !tui.running {
		return
	}

	for _, p := range tui.layout.panes() {
		tui.paint(p.x, p.y, p.rows)
	}

	if pos := tui.cursorpos; pos != nil {
		tui.screen.ShowCursor(pos[0], pos[1])
	} else {
		tui.screen.HideCursor()
	}

	tui.screen.Show()
}

func (tui *TUI) paint(x, y int, rows Rows) {
	for yy, row := range rows {
		for xx, cell := range row {
			tui.screen.SetContent(
				x+xx, y+yy,
				cell.Content, nil, cell.Style,
			)
		}
	}
}

const (
	keyNumEnter tcell.Key = iota + 1024
	keyNumEqual
	keyNumMulti
	keyNumPlus
	keyNumMinus
	keyNumDot
	keyNumDiv
	keyNum0
	keyNum1
	keyNum2
	keyNum3
	keyNum4
	keyNum5
	keyNum6
	keyNum7
	keyNum8
	keyNum9
)

var numpadKeys = map[int]tcell.Key{
	77:  keyNumEnter,
	88:  keyNumEqual,
	106: keyNumMulti,
	107: keyNumPlus,
	109: keyNumMinus,
	110: keyNumDot,
	111: keyNumDiv,
	112: keyNum0,
	113: keyNum1,
	114: keyNum2,
	115: keyNum3,
	116: keyNum4,
	117: keyNum5,
	118: keyNum6,
	119: keyNum7,
	120: keyNum8,
	121: keyNum9,
}

func isNumpad(ev *tcell.EventKey) bool {
	return ev.Key() == tcell.KeyRune &&
		ev.Rune() == 'O' &&
		ev.Modifiers() == tcell.ModAlt
}

func makeNumpad(ev *tcell.EventKey) *tcell.EventKey {
	if key, ok := numpadKeys[int(ev.Rune())]; ok {
		return tcell.NewEventKey(key, 0, 0)
	}
	return ev
}
