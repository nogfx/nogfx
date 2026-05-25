package achaea

import (
	"strings"

	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/app/connection"
	"github.com/nogfx/nogfx/app/ui"
	"github.com/nogfx/nogfx/internal/simpex"
)

// TunnelVision filters server output to make large quantities of combat
// text easier and faster to read.
//
// It omits common spam (balance-recovery confirmations, queue commands,
// weather, defence acquisitions), suppresses paired curing lines (the
// action message followed by the effect message), and consolidates a
// flurry of attack lines (the strike plus its modifiers — hit, miss,
// dodge, critical, …) into a single coloured summary.
//
// Attack consolidation buffers attack and modifier lines across batches
// (each TextLine arrives in its own batch) and emits the summary as a
// ui.PrintLine command when the run ends — on the next non-attack TextLine
// or on a Prompt. The interleaved GMCP frames a typical Achaea response
// carries do not break a run.
type TunnelVision struct {
	character *Character

	// expectCured is set when the previous TextLine was a tvCuring; the
	// next batch checks whether its event is the matching tvCured and, if
	// so, drops it too.
	expectCured bool

	// attackTarget is the target name captured from the first attack line
	// in the run; attackParts is the running sequence of attack-name and
	// modifier tokens accumulated so far.
	attackTarget string
	attackParts  []string
}

// rewriteProcessor classifies the trigger event and either drops it
// (omits, paired curing/cured, attack and modifier lines being buffered)
// or passes it through. When an attack run ends — on a non-attack TextLine
// or on a Prompt — the consolidated summary is emitted as a ui.PrintLine
// command appended to the same batch, so the engine prints the summary
// just before whatever event ended the run.
func (tv *TunnelVision) rewriteProcessor() app.Processor {
	return func(batch app.Batch) (app.Batch, error) {
		switch ev := batch.Event.(type) {
		case connection.TextLine:
			return tv.handleTextLine(batch, ev), nil
		case connection.Prompt:
			tv.expectCured = false

			return tv.flushInto(batch), nil
		default:
			tv.expectCured = false

			return batch, nil
		}
	}
}

func (tv *TunnelVision) handleTextLine(batch app.Batch, line connection.TextLine) app.Batch {
	c := classifyTunnelVision(line.Bytes, tv.character)

	// Attack and modifier lines accumulate into the run and are dropped;
	// the flush at run-end replaces them with a single summary line.
	if c.kind == tvAttack || c.kind == tvAttackModifier {
		if c.kind == tvAttack && tv.attackTarget == "" {
			tv.attackTarget = c.detail
		}

		if c.kind == tvAttack {
			tv.attackParts = append(tv.attackParts, "\x1b[32;1m"+c.style+"\x1b[0m")
		} else {
			tv.attackParts = append(tv.attackParts, c.detail)
		}

		tv.expectCured = false
		batch.Event = nil

		return batch
	}

	// Any other classification ends an in-flight attack run.
	batch = tv.flushInto(batch)

	if tv.expectCured {
		tv.expectCured = false

		if c.kind == tvCured {
			batch.Event = nil

			return batch
		}
	}

	switch c.kind {
	case tvOmit:
		batch.Event = nil
	case tvCuring:
		tv.expectCured = true
		batch.Event = nil
	case tvNone, tvCured:
		// Pass through. Stray cured lines (without a preceding curing)
		// fall here too.
	case tvAttack, tvAttackModifier:
		// Unreachable: attack kinds returned above.
	}

	return batch
}

// flushInto appends a consolidated summary PrintLine command for any
// in-flight attack run and clears the accumulator. With nothing buffered
// it returns the batch unchanged.
func (tv *TunnelVision) flushInto(batch app.Batch) app.Batch {
	if len(tv.attackParts) == 0 {
		return batch
	}

	var b strings.Builder
	b.WriteString("You \x1b[32;1mattack\x1b[0m")

	if tv.attackTarget != "" {
		b.WriteByte(' ')
		b.WriteString(tv.attackTarget)
	}

	b.WriteString(" / ")
	b.WriteString(strings.Join(tv.attackParts, " "))
	b.WriteByte('.')

	tv.attackTarget = ""
	tv.attackParts = nil

	bs := []byte(b.String())

	return batch.AppendEffect(ui.PrintLine{
		Line: ui.Line{Raw: bs, Formatted: bs},
	})
}

// tvClass holds the classification of a single TextLine.
type tvClass struct {
	kind   tvKind
	style  string // for attacks: the attack name (Sidekick, Uppercut, …)
	detail string // for attacks: the target name; for modifiers: rendered text
}

type tvKind int

const (
	tvNone tvKind = iota
	tvOmit
	tvAttack
	tvAttackModifier
	tvCuring
	tvCured
)

// tvOmits are output lines suppressed outright.
var tvOmits = [][]byte{
	// Balance-recovery confirmations.
	[]byte("You may drink another health or mana elixir."),
	[]byte("You may eat another bit of irid moss or potash."),
	[]byte("You may apply another salve to yourself."),

	// Defences acquired (their acquisition messages).
	[]byte("You shut your eyes and concentrate on the Soulrealms. A moment later, you feel inextricably linked with the realm of Death."),
	[]byte("Your vision sharpens with light as you gain night sight."),
	[]byte("Your body begins to feel lighter and you feel that you are floating slightly."),
	[]byte("A brief shiver runs through your body."),
	[]byte("Flexing your muscles, you concentrate on forcing unnatural toughness over the surface of your skin."),

	// Queue activity.
	[]byte("[System]: Queued ^ commands cleared."),
	[]byte("[System]: Added * to your ^ queue."),
	[]byte("[System]: Running queued ^ command: *"),

	// Weather chatter.
	[]byte("Occasional drops of rain fall to the ground from a sky grey with pregnant clouds."),
	[]byte("Occasional raindrops fall on your head as the drizzle continues."),

	// Special clot.
	[]byte("You exert superior mental control and your wounds clot before your eyes."),
}

// tvCuringPatterns are "I just used a cure" lines (the action).
var tvCuringPatterns = [][]byte{
	[]byte("You take a drink from *."),
	[]byte("You down the last drop from *."),
	[]byte("You eat a potash crystal."),
	[]byte("You take out some salve and quickly rub it on your skin."),
}

// tvCuredPatterns are "the cure worked" lines (the effect).
var tvCuredPatterns = [][]byte{
	[]byte("The elixir heals and soothes you."),
	[]byte("Your mind feels stronger and more alert."),
	[]byte("You feel your health and mana replenished."),
	[]byte("A feeling of comfortable warmth spreads over you."),
}

// tvAttacks maps attack names to their first-person trigger lines. Used to
// recognise our own attack and start an "attacks summary" line.
var tvAttacks = map[string]string{
	// Tekura / monk attacks.
	"Kick":        "You leap into the air and launch a flying kick at {*}.",
	"Axe":         "You kick your leg high and scythe downwards at {*}.",
	"Backbreaker": "You move in towards {*} for the backbreaker.",
	"Bladehand":   "You whip your hand in a vicious bladehand at the neck of {*}.",
	"Hammerfist":  "You ball up one fist and hammerfist {*}.",
	"Hook":        "You unleash a powerful hook towards {*}.",
	"Jab":         "You expertly jab your fingers into the nerve cluster behind the ear of {*}.",
	"Jumpkick":    "Your foot slams into {*}, knocking ^ off ^ feet.",
	"Moonkick":    "You hurl yourself towards {*} with a lightning-fast moon kick.",
	"Palmstrike":  "You throw your force behind a forward palmstrike at {*}'s face.",
	"Roundhouse":  "You twist your torso and send a roundhouse towards {*}.",
	"Sidekick":    "You pump out at {*} with a powerful side kick.",
	"Slam":        "You move in to grab {*} for a body slam.",
	"Snapkick":    "You let fly at {*} with a snap kick.",
	"Spear":       "You form a spear hand and stab out towards {*}.",
	"Sweepkick":   "You drop to the floor and sweep your legs round at {*}.",
	"Thrustkick":  "You thrust your leg out towards {*} with considerable force.",
	"Uppercut":    "You launch a powerful uppercut at {*}.",
	"Whirlwind":   "You spin into the air and throw a whirlwind kick towards {*}.",
	"Wrench":      "Stepping forward, you grab the useless ^ arm of {*}, wrench ^ violently forward, and throw ^ to the ground.",
}

// tvModifiers maps attack-modifier patterns to the short label rendered in
// the summary line.
var tvModifiers = map[string]string{
	"You miss.":                                                 "\x1b[33mmiss\x1b[0m",
	"You connect!":                                              "hit",
	"You connect to the ^!":                                     "hit",
	"You connect to the ^ ^!":                                   "hit",
	"You have scored a CRITICAL hit!":                           "x2",
	"You have scored a CRUSHING CRITICAL hit!":                  "x4",
	"You have scored an OBLITERATING CRITICAL hit!":             "x8",
	"You have scored an ANNIHILATINGLY POWERFUL CRITICAL hit!":  "\x1b[32mx16\x1b[0m",
	"You have scored a WORLD-SHATTERING CRITICAL HIT!!!":        "\x1b[1;32mx32\x1b[0m",
	"You kick scythes through nothing, hitting only empty air.": "\x1b[33munshielded\x1b[0m",
	"* twists ^ body out of harm's way.":                        "\x1b[33mdodge\x1b[0m",
	"* backs away and out of your reach.":                       "\x1b[33mdodge\x1b[0m",
	"A reflection of ^ blinks out of existence.":                "\x1b[33mreflection\x1b[0m",
	"* stands firm and does not budge against the thrust kick.": "\x1b[33msturdiness\x1b[0m",
	"* ceases tending to ^ wounds.":                             "awoke",
}

func classifyTunnelVision(text []byte, _ *Character) tvClass {
	for _, p := range tvOmits {
		if simpex.Match(p, text) != nil {
			return tvClass{kind: tvOmit}
		}
	}

	for _, p := range tvCuringPatterns {
		if simpex.Match(p, text) != nil {
			return tvClass{kind: tvCuring}
		}
	}

	for _, p := range tvCuredPatterns {
		if simpex.Match(p, text) != nil {
			return tvClass{kind: tvCured}
		}
	}

	for name, pattern := range tvAttacks {
		caps := simpex.Match([]byte(pattern), text)
		if caps != nil {
			target := ""
			if len(caps) > 0 {
				target = string(caps[0])
			}

			return tvClass{kind: tvAttack, style: name, detail: target}
		}
	}

	for pattern, label := range tvModifiers {
		if simpex.Match([]byte(pattern), text) != nil {
			return tvClass{kind: tvAttackModifier, detail: label}
		}
	}

	return tvClass{kind: tvNone}
}
