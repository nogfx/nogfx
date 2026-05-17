package achaea_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/nogfx/nogfx/worlds/achaea"
)

// TestTargetingLogic exercises the auto-retarget machinery on Target
// directly (candidates, present, enter/leave, replacing candidates) and
// checks the "settarget …" sends it queues. Name itself is not asserted
// here — the server confirms the switch via IRETargetSet, which is
// covered by TestWorldTargeting.
func TestTargetingLogic(t *testing.T) {
	tcs := map[string]struct {
		candidates    []string
		present       []string
		enters        []string
		leaves        []string
		newcandidates []string
		sets          []string
	}{
		"one present": {
			candidates: []string{"one", "two"},
			present:    []string{"a one thing"},
			sets:       []string{"settarget one"},
		},

		"two present": {
			candidates: []string{"one", "two"},
			present:    []string{"a two thing"},
			sets:       []string{"settarget two"},
		},

		"three present": {
			candidates: []string{"one", "two"},
			present:    []string{"a three thing"},
		},

		"two present one enters": {
			candidates: []string{"one", "two"},
			present:    []string{"a two thing"},
			enters:     []string{"a one thing"},
			sets:       []string{"settarget two", "settarget one"},
		},

		"one present two enters": {
			candidates: []string{"one", "two"},
			present:    []string{"a one thing"},
			enters:     []string{"a two thing"},
			sets:       []string{"settarget one"},
		},

		"replacing candidates clears non-matching name": {
			candidates:    []string{"one", "two"},
			present:       []string{"a one thing"},
			newcandidates: []string{"three"},
			sets:          []string{"settarget one"},
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			tgt := achaea.NewTarget()

			if tc.candidates != nil {
				tgt.SetCandidates(tc.candidates)
			}
			if tc.present != nil {
				tgt.SetPresent(tc.present)
			}
			for _, name := range tc.enters {
				tgt.AddPresent(name)
			}
			for _, name := range tc.leaves {
				tgt.RemovePresent(name)
			}
			if tc.newcandidates != nil {
				tgt.SetCandidates(tc.newcandidates)
			}

			var sent []string
			for _, b := range tgt.DrainSends() {
				sent = append(sent, string(b))
			}
			assert.Equal(t, tc.sets, sent)
		})
	}
}
