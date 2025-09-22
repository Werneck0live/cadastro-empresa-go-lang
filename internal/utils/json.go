package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

func WriteJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

/*
decodeStrict decodifica JSON rejeitando chaves desconhecidas
e garantindo que exista exatamente UM objeto JSON.
*/
func DecodeStrict(r io.Reader, dst any) error {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		/*
			retorna mensagens refenre ao erro dos campos,
			como por exemplo (ex.: "json: unknown field \"foo\"")
		*/
		return err
	}
	/*
		Garante que não tenha lixo após o objeto JSON
		Uma forma de checar EOF seria tentar um segundo Decode em struct{} e exigir EOF.
	*/
	if dec.More() {
		return errors.New("unexpected additional JSON content")
	}

	return nil
}

func BadRequest(w http.ResponseWriter, msg string) {
	WriteJSON(w, http.StatusBadRequest, map[string]string{"error": msg})
}

// O tipo "err" custmoza as mensagens de erro "unknown field"
func FormatUnknownFieldError(err error) string {
	// Os erros do stdlib já vêm bons, mas se quiser customizar, faça aqui.

	return fmt.Sprintf("%v", err)
}
