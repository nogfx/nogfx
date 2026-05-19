package tui

import (
	"math"
	"strconv"

	"github.com/gdamore/tcell/v2"
)

var (
	// Predefined styles of some common vitals. i=0 is full, i=1 is empty.
	vitalStyles = map[string][]tcell.Style{
		// Fallback style, for when none other matches.
		"": {
			tcell.StyleDefault.
				Background(tcell.Color250).
				Foreground(tcell.ColorBlack),
			tcell.StyleDefault.
				Background(tcell.Color240).
				Foreground(tcell.ColorBlack),
		},
		"health": {
			tcell.StyleDefault.
				Background(tcell.ColorGreen).
				Foreground(tcell.ColorBlack),
			tcell.StyleDefault.
				Background(tcell.ColorDarkGreen).
				Foreground(tcell.ColorBlack),
		},
		"mana": {
			tcell.StyleDefault.
				Background(tcell.ColorBlue).
				Foreground(tcell.ColorBlack),
			tcell.StyleDefault.
				Background(tcell.ColorDarkBlue).
				Foreground(tcell.ColorBlack),
		},
		"endurance": {
			tcell.StyleDefault.
				Background(tcell.ColorTeal).
				Foreground(tcell.ColorBlack),
			tcell.StyleDefault.
				Background(tcell.ColorDarkCyan).
				Foreground(tcell.ColorBlack),
		},
		"willpower": {
			tcell.StyleDefault.
				Background(tcell.ColorFuchsia).
				Foreground(tcell.ColorBlack),
			tcell.StyleDefault.
				Background(tcell.ColorRebeccaPurple).
				Foreground(tcell.ColorBlack),
		},
		"energy": {
			tcell.StyleDefault.
				Background(tcell.ColorYellow).
				Foreground(tcell.ColorBlack),
			tcell.StyleDefault.
				Background(tcell.Color100).
				Foreground(tcell.ColorBlack),
		},
		"target": {
			tcell.StyleDefault.
				Background(tcell.ColorRed).
				Foreground(tcell.ColorBlack),
			tcell.StyleDefault.
				Background(tcell.ColorDarkRed).
				Foreground(tcell.ColorBlack),
		},
	}
)

// RenderVitals renders the current Vitals.
func (tui *TUI) RenderVitals(width int) Rows {
	if rows, ok := tui.getCache(paneVitals); ok {
		return rows
	}

	tui.vitalsMu.Lock()
	order := tui.collectVitalsOrder()

	values := make([]*vital, 0, len(order))
	for _, name := range order {
		values = append(values, tui.lookupVital(name))
	}
	tui.vitalsMu.Unlock()

	if len(order) == 0 {
		return Rows{}
	}

	gapStyle := (tcell.Style{}).Background(tcell.Color235)

	row := Row{}

	for i, name := range order {
		styles, ok := vitalStyles[name]
		if !ok {
			styles = vitalStyles[""]
		}

		row = row.append(NewRow(min(1, i), NewCell(' ', gapStyle))...)
		row = row.append(RenderVital(
			values[i],
			(width-len(row))/(len(order)-i),
			styles,
		)...)
	}

	rows := Rows{row}

	tui.setCache(paneVitals, rows)

	return rows
}

// collectVitalsOrder builds the display order: health, mana, then any
// auxiliary vitals in registration order. The caller must hold vitalsMu.
func (tui *TUI) collectVitalsOrder() []string {
	var order []string
	if tui.health != nil {
		order = append(order, "health")
	}

	if tui.mana != nil {
		order = append(order, "mana")
	}

	order = append(order, tui.vitalsOrder...)

	return order
}

// lookupVital returns the vital with the given name. The caller must hold
// vitalsMu.
func (tui *TUI) lookupVital(name string) *vital {
	switch name {
	case "health":
		return tui.health
	case "mana":
		return tui.mana
	default:
		return tui.vitalsByName[name]
	}
}

// RenderVital renders the given vital.
func RenderVital(v *vital, width int, styles []tcell.Style) Row {
	if v == nil || v.Max == 0 {
		empty := NewRow(width, NewCell(' ', styles[1]))

		return empty
	}

	fullWidth := int(math.Round(
		(float64(width) * float64(v.Value) / float64(v.Max)) - 0.01,
	))

	full := NewRow(fullWidth, NewCell(' ', styles[0]))
	empty := NewRow(width-len(full), NewCell(' ', styles[1]))

	row := make(Row, 0, len(full)+len(empty))
	row = append(row, full...)
	row = append(row, empty...)

	value := strconv.Itoa(v.Value)

	if len(value) <= len(row) {
		for i, x := 0, (width-len(value))/2; i < len(value); i++ {
			row[x+i].Content = rune(value[i])
		}
	}

	return row
}
