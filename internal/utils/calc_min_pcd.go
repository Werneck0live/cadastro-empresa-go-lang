package utils

import "math"

// computeMinPCD retorna o mínimo de PcDs exigidos pela Lei 8.213/91 (art. 93).
// Regra: <100 -> 0; 100–200 -> 2%; 201–500 -> 3%; 501–1000 -> 4%; 1001+ -> 5%.
// Arredondamento: sempre para cima (ceil) quando fracionar
func ComputeMinPCD(total int) int {
	if total < 100 {
		return 0
	}
	var p float64
	switch {
	case total <= 200:
		p = 0.02
	case total <= 500:
		p = 0.03
	case total <= 1000:
		p = 0.04
	default:
		p = 0.05
	}
	return int(math.Ceil(float64(total) * p))
}
