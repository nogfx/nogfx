package tui

import (
	"math"
)

func abs(a int) int {
	return int(math.Abs(float64(a)))
}

func rel(a, b int) int {
	return abs(a) * max(-1, min(1, b))
}

func proc(whole, part, parts int) int {
	return int(float64(whole) * float64(part) / float64(parts))
}
