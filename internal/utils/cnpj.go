package utils

import "unicode"

// remove qualquer coisa que não seja dígito
func SanitizeCNPJ(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if unicode.IsDigit(r) {
			out = append(out, r)
		}
	}
	return string(out)
}

// Para o desafio, decidi validar SÓ de qtd de caract (14) e todos dígitos iguais
func ValidateCNPJ(cnpj string) bool {
	if len(cnpj) != 14 {
		return false
	}
	allEq := true
	for i := 1; i < 14; i++ {
		if cnpj[i] != cnpj[0] {
			allEq = false
			break
		}
	}
	return !allEq
}
