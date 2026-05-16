package achaea

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/tobiassjosten/nogfx/app"
	"github.com/tobiassjosten/nogfx/connection"
	"github.com/tobiassjosten/nogfx/lib/simpex"
)

// TunnelVision filters and rewrites server output to make large quantities
// of combat text easier and faster to read.
//
// It does two things to each paragraph of output:
//
//   - Omit common spam — balance-recovery confirmations, queue commands,
//     weather, defence acquisitions — that the user does not need to see.
//   - Consolidate a flurry of attack-related lines (the strike, the
//     critical, the dodge/miss/reflect) into a single coloured summary.
//
// The classifier is pattern-driven and lives in tvPatterns / tvOmits /
// tvModifiers. Lines that match none of those patterns pass through
// unchanged.
type TunnelVision struct {
	character *Character
}

// rewriteProcessor is the post-classification rewrite pass. It walks the
// batch's TextLine events in order, drops the ones that should be omitted,
// and consolidates consecutive attack lines into a single summary line.
func (tv TunnelVision) rewriteProcessor() app.Processor {
	return func(batch app.Batch) (app.Batch, error) {
		var (
			out      []app.Event
			attacks  []string
			attackOn string
			i        int
		)

		flush := func() {
			if len(attacks) == 0 {
				return
			}
			text := "You \x1b[32;1mattack\x1b[0m"
			if attackOn != "" {
				text += " " + attackOn
			}
			text += " " + strings.Join(attacks, " ")
			out = append(out, connection.TextLine{Bytes: []byte(text + ".")})
			attacks = nil
			attackOn = ""
		}

		// Pre-pass: build a parallel slice marking each TextLine event with
		// classification info. Non-TextLine events keep their slot empty.
		classes := make([]tvClass, len(batch.Events))
		for i, ev := range batch.Events {
			line, ok := ev.(connection.TextLine)
			if !ok {
				continue
			}
			classes[i] = classifyTunnelVision(line.Bytes, tv.character)
		}

		// Paired curing suppression: drop a "you took a cure" line when it
		// is immediately followed by the matching "you feel cured" line, or
		// vice versa.
		for i := 0; i < len(classes); i++ {
			if classes[i].kind == tvCuring && i+1 < len(classes) && classes[i+1].kind == tvCured {
				classes[i].kind = tvOmit
				classes[i+1].kind = tvOmit
				i++
			}
		}

		for i = 0; i < len(batch.Events); i++ {
			ev := batch.Events[i]
			c := classes[i]

			switch c.kind {
			case tvOmit:
				continue

			case tvAttack:
				if attackOn == "" {
					attackOn = "\x1b[32;1m" + c.detail + "\x1b[0m"
				}
				attacks = append(attacks, "/ \x1b[32;1m"+c.style+"\x1b[0m")

			case tvAttackModifier:
				attacks = append(attacks, c.detail)

			default:
				flush()
				out = append(out, ev)
			}
		}
		flush()

		batch.Events = out
		return batch, nil
	}
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
	"You miss.":                                            "\x1b[33mmiss\x1b[0m",
	"You connect!":                                         "hit",
	"You connect to the ^!":                                "hit",
	"You connect to the ^ ^!":                              "hit",
	"You have scored a CRITICAL hit!":                      "x2",
	"You have scored a CRUSHING CRITICAL hit!":             "x4",
	"You have scored an OBLITERATING CRITICAL hit!":        "x8",
	"You have scored an ANNIHILATINGLY POWERFUL CRITICAL hit!": "\x1b[32mx16\x1b[0m",
	"You have scored a WORLD-SHATTERING CRITICAL HIT!!!":   "\x1b[1;32mx32\x1b[0m",
	"You kick scythes through nothing, hitting only empty air.": "\x1b[33munshielded\x1b[0m",
	"* twists ^ body out of harm's way.":                   "\x1b[33mdodge\x1b[0m",
	"* backs away and out of your reach.":                  "\x1b[33mdodge\x1b[0m",
	"A reflection of ^ blinks out of existence.":           "\x1b[33mreflection\x1b[0m",
	"* stands firm and does not budge against the thrust kick.": "\x1b[33msturdiness\x1b[0m",
	"* ceases tending to ^ wounds.":                        "awoke",
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

// stripStyle is exported for tests so they can compare against the visible
// portion of a styled summary line.
func stripStyle(text []byte) []byte {
	var out []byte
	for i := 0; i < len(text); i++ {
		if text[i] == 0x1b && i+1 < len(text) && text[i+1] == '[' {
			j := i + 2
			for j < len(text) && text[j] != 'm' {
				j++
			}
			i = j
			continue
		}
		out = append(out, text[i])
	}
	return bytes.TrimSpace(out)
}

// formatCount is unused at present but reserved for future use when
// TunnelVision reports e.g. "5 misses" instead of repeating "miss".
//
//nolint:unused
func formatCount(n int, label string) string {
	if n <= 1 {
		return label
	}
	return fmt.Sprintf("%dx %s", n, label)
}
