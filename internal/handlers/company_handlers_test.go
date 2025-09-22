package handlers

/*
RODAR TODOS OS TESTES:

go test -run 'TestCompanies_List_|TestCompanyByID_Get_|TestCompanies_Create_|TestCompanyByID_Put_|TestCompanyByID_Patch_|TestCompanyByID_Delete_' -v ./internal/handlers -count=1

*/

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Werneck0live/cadastro-empresa/internal/models"
	"github.com/Werneck0live/cadastro-empresa/internal/repository"
	"github.com/Werneck0live/cadastro-empresa/internal/utils"

	amqp091 "github.com/rabbitmq/amqp091-go"
)

const validCNPJ = "11.222.333/0001-81"
const companyID = "11222333000181"      // corresponde ao 11.222.333/0001-81
const otherValidCNPJ = "76986532000101" // corresponde ao 11.222.333/0001-81

// 1)  GET (ListAll) - go test -run 'TestCompanies_List_' -v ./internal/handlers -count=1

func TestCompanies_List(t *testing.T) {

	rm := &repoMock{
		GetAllFn: func(_ context.Context, limit, skip int64) ([]models.Company, error) {
			// valida se o handler aplicou corretamente os query params
			if limit != 10 || skip != 0 {
				t.Fatalf("params: want limit=10, skip=0; got limit=%d skip=%d", limit, skip)
			}
			return []models.Company{
				{ID: "12345678000190", CNPJ: "12.345.678/0001-90", NomeFantasia: "ACME"},
			}, nil
		},
	}

	pm := &pubMock{
		PublishFn: func(_ context.Context, _ string, _ amqp091.Table) error { return nil },
	}

	h := &CompanyHandler{Repo: rm, Pub: pm}

	req := httptest.NewRequest(http.MethodGet, "/api/companies?limit=10&skip=0", nil)
	rr := httptest.NewRecorder()

	h.Companies(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d; want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var got []models.Company
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid json: %v\nbody=%s", err, rr.Body.String())
	}
	if len(got) != 1 || got[0].NomeFantasia != "ACME" {
		t.Fatalf("unexpected payload: %#v", got)
	}
}

/*
Validação de faixa (limit inválido - handler deve cair no default e não quebrar).
Ex.: limit=9999 (o código aceita até 200).
*/
func TestCompanies_List_LimitParsing(t *testing.T) {
	cases := []struct {
		name      string
		q         string
		wantLimit int64
	}{
		{"default", "", 50},
		{"valid_min", "?limit=1", 1},
		{"valid_mid", "?limit=73", 73},
		{"valid_max", "?limit=200", 200},
		{"too_large", "?limit=9999", 50},
		{"zero", "?limit=0", 50},
		{"negative", "?limit=-10", 50},
		{"non_numeric", "?limit=abc", 50},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rm := &repoMock{
				GetAllFn: func(_ context.Context, limit, skip int64) ([]models.Company, error) {
					if limit != tc.wantLimit {
						t.Fatalf("want limit=%d got=%d", tc.wantLimit, limit)
					}
					return nil, nil
				},
			}
			h := &CompanyHandler{Repo: rm, Pub: &pubMock{}}
			req := httptest.NewRequest(http.MethodGet, "/api/companies"+tc.q, nil)
			rr := httptest.NewRecorder()
			h.Companies(rr, req)
			if rr.Code != http.StatusOK {
				t.Fatalf("status=%d want=%d", rr.Code, http.StatusOK)
			}
		})
	}
}

// Erro do repositório (500)
func TestCompanies_List_RepoError(t *testing.T) {
	rm := &repoMock{
		GetAllFn: func(_ context.Context, _, _ int64) ([]models.Company, error) {
			return nil, errors.New("boom")
		},
	}
	h := &CompanyHandler{Repo: rm, Pub: &pubMock{}}

	req := httptest.NewRequest(http.MethodGet, "/api/companies", nil)
	rr := httptest.NewRecorder()
	h.Companies(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d want=%d body=%s", rr.Code, http.StatusInternalServerError, rr.Body.String())
	}
}

// Method Not Allowed (405)
func TestCompanies_MethodNotAllowed(t *testing.T) {
	h := &CompanyHandler{Repo: &repoMock{}, Pub: &pubMock{}}
	req := httptest.NewRequest(http.MethodDelete, "/api/companies", nil)
	rr := httptest.NewRecorder()
	h.Companies(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d want=%d", rr.Code, http.StatusMethodNotAllowed)
	}
}

// 2) GET (ById{id}) - go test -run 'TestCompanyByID_Get_' -v ./internal/handlers -count=1

// ---------- 200 OK (found)
func TestCompanyByID_Get_Found(t *testing.T) {
	rm := &repoMock{
		GetByIDFn: func(_ context.Context, id string) (*models.Company, error) {
			if id != companyID {
				t.Fatalf("id inesperado: got=%s want=%s", id, companyID)
			}
			return &models.Company{
				ID:           id,
				CNPJ:         "11.222.333/0001-81",
				NomeFantasia: "ACME",
			}, nil
		},
	}
	h := &CompanyHandler{Repo: rm, Pub: &pubMock{}}

	req := httptest.NewRequest(http.MethodGet, "/api/companies/"+companyID, nil)
	rr := httptest.NewRecorder()

	h.CompanyByID(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d want=%d body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var got models.Company
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("json inválido: %v (body=%s)", err, rr.Body.String())
	}
	if got.ID != companyID || got.NomeFantasia != "ACME" {
		t.Fatalf("payload inesperado: %#v", got)
	}
}

// ---------- 404 Not Found (repo devolve erro)
func TestCompanyByID_Get_NotFound(t *testing.T) {
	rm := &repoMock{
		GetByIDFn: func(_ context.Context, id string) (*models.Company, error) {
			return nil, errors.New("not found")
		},
	}
	h := &CompanyHandler{Repo: rm, Pub: &pubMock{}}

	req := httptest.NewRequest(http.MethodGet, "/api/companies/"+companyID, nil)
	rr := httptest.NewRecorder()

	h.CompanyByID(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status=%d want=%d body=%s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
}

// ---------- 404 Not Found (path sem id -> parseIDFromPath falha)
func TestCompanyByID_Get_InvalidPath(t *testing.T) {
	h := &CompanyHandler{Repo: &repoMock{}, Pub: &pubMock{}}

	req := httptest.NewRequest(http.MethodGet, "/api/companies/", nil) // sem ID no final
	rr := httptest.NewRecorder()

	h.CompanyByID(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status=%d want=%d body=%s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
}

// 3) POST (create) - go test run 'TestCompanies_Create_' -v ./internal/handlers -count=1

// CNPJ válido (com dígitos verificadores corretos): 11.222.333/0001-81

// ---------- 201 CREATED (payload válido)
func TestCompanies_Create_Valid(t *testing.T) {
	rm := &repoMock{
		CreateFn: func(_ context.Context, c *models.Company) (string, error) {
			// sanity check
			if c.CNPJ == "" || c.NomeFantasia == "" {
				t.Fatal("campos obrigatórios não chegaram no repo")
			}
			return c.CNPJ, nil
		},
	}
	pm := &pubMock{PublishFn: func(_ context.Context, _ string, _ amqp091.Table) error { return nil }}

	h := &CompanyHandler{Repo: rm, Pub: pm}

	body := bytes.NewBufferString(`{
		"cnpj": "` + validCNPJ + `",
		"nome_fantasia": "ACME",
		"razao_social": "ACME S.A.",
		"endereco": "Rua X, 123",
		"numero_funcionarios": 50
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/companies", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.Companies(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status=%d want=%d body=%s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	// (opcional) valida o JSON retornado
	var got models.Company
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("json inválido: %v", err)
	}
	if got.CNPJ == "" || got.ID == "" {
		t.Fatalf("payload inesperado: %#v", got)
	}

	if got.NumeroMinimoPCDExigidos != utils.ComputeMinPCD(got.NumeroMinimoPCDExigidos) {
		t.Fatalf("pcd incorreto: got=%d", got.NumeroMinimoPCDExigidos)
	}
}

// ---------- 400 BAD REQUEST (JSON inválido)
func TestCompanies_Create_InvalidJSON(t *testing.T) {
	h := &CompanyHandler{Repo: &repoMock{}, Pub: &pubMock{}}

	// JSON quebrado - bytes.NewBufferString(`{`)
	req := httptest.NewRequest(http.MethodPost, "/api/companies", bytes.NewBufferString(`{`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.Companies(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want=%d body=%s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
}

// ---------- 400 BAD REQUEST (CNPJ inválido)
func TestCompanies_Create_InvalidCNPJ(t *testing.T) {
	h := &CompanyHandler{Repo: &repoMock{}, Pub: &pubMock{}}

	// cnpj inválido
	body := bytes.NewBufferString(`{
		"cnpj": "xx", 
		"nome_fantasia": "ACME"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/companies", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.Companies(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want=%d body=%s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
}

// ---------- 409 CONFLICT (CNPJ duplicado)
func TestCompanies_Create_DuplicateCNPJ(t *testing.T) {
	rm := &repoMock{
		CreateFn: func(_ context.Context, _ *models.Company) (string, error) {
			return "", repository.ErrDuplicateCNPJ
		},
	}
	h := &CompanyHandler{Repo: rm, Pub: &pubMock{}}

	body := bytes.NewBufferString(`{
		"cnpj": "` + validCNPJ + `",
		"nome_fantasia": "ACME"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/companies", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.Companies(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("status=%d want=%d body=%s", rr.Code, http.StatusConflict, rr.Body.String())
	}
}

// 4) PUT - (replace) - go test -run 'TestCompanyByID_Put_' -v ./internal/handlers -count=1
// ---------- 200 OK (replace válido)
func TestCompanyByID_Put_Replace_OK(t *testing.T) {
	rm := &repoMock{
		GetByIDFn: func(_ context.Context, id string) (*models.Company, error) {
			// retorna o atual existente (para o handler preservar CreatedAt)
			return &models.Company{ID: id, CNPJ: validCNPJ, NomeFantasia: "Old", CreatedAt: time.Now().Add(-time.Hour)}, nil
		},
		ReplaceFn: func(_ context.Context, id string, doc *models.Company) error {
			// sanity checks
			if id != companyID {
				t.Fatalf("id inesperado em Replace: got=%s want=%s", id, companyID)
			}
			if doc.ID != companyID || doc.CNPJ == "" || doc.NomeFantasia != "ACME NEW" {
				t.Fatalf("doc inesperado: %#v", doc)
			}
			return nil
		},
	}
	h := &CompanyHandler{Repo: rm, Pub: &pubMock{}}

	body := bytes.NewBufferString(`{
		"cnpj": "` + validCNPJ + `",
		"nome_fantasia": "ACME NEW",
		"razao_social": "ACME S.A.",
		"endereco": "Rua Y, 456",
		"numero_funcionarios": 80
	}`)
	req := httptest.NewRequest(http.MethodPut, "/api/companies/"+companyID, body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.CompanyByID(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d want=%d body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	var got models.Company
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("json inválido: %v", err)
	}
	if got.ID != companyID || got.NomeFantasia != "ACME NEW" {
		t.Fatalf("payload inesperado: %#v", got)
	}
}

// ---------- 400 BAD REQUEST (cnpj do body diferente do {id})
func TestCompanyByID_Put_CNPJMismatch(t *testing.T) {
	idCNPJ := validCNPJ        // CNPJ do recurso na URL (válido)
	bodyCNPJ := otherValidCNPJ // outro CNPJ válido e diferente do idCNPJ

	// mock do repo: recurso EXISTE (para não cair em 404)
	rm := &repoMock{
		GetByIDFn: func(_ context.Context, id string) (*models.Company, error) {
			if id != utils.SanitizeCNPJ(idCNPJ) {
				t.Fatalf("id inesperado: %s", id)
			}
			return &models.Company{
				ID:   utils.SanitizeCNPJ(idCNPJ),
				CNPJ: utils.SanitizeCNPJ(idCNPJ),
			}, nil
		},
		// Replace NÃO deve ser chamado, pois o handler retorna 400 antes
		ReplaceFn: func(_ context.Context, _ string, _ *models.Company) error {
			t.Fatalf("Replace não deveria ser chamado em caso de mismatch")
			return nil
		},
	}

	h := &CompanyHandler{Repo: rm, Pub: &pubMock{}}

	body := bytes.NewBufferString(fmt.Sprintf(`{
        "cnpj": "%s",
        "nome_fantasia": "ACME",
        "razao_social": "ACME LTDA",
        "endereco": "Rua X",
        "numero_funcionarios": 123
    }`, bodyCNPJ))

	req := httptest.NewRequest(http.MethodPut, "/api/companies/"+utils.SanitizeCNPJ(idCNPJ), body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.CompanyByID(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want=%d body=%s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "cnpj in body must match") {
		t.Fatalf("esperava mensagem de mismatch; body=%s", rr.Body.String())
	}
}

// ---------- 404 Not Found (GetByID inicial não acha)
func TestCompanyByID_Put_NotFoundCurrent(t *testing.T) {
	rm := &repoMock{
		GetByIDFn: func(_ context.Context, _ string) (*models.Company, error) {
			return nil, errors.New("not found")
		},
	}
	h := &CompanyHandler{Repo: rm, Pub: &pubMock{}}
	body := bytes.NewBufferString(`{"nome_fantasia":"ACME"}`)

	req := httptest.NewRequest(http.MethodPut, "/api/companies/"+companyID, body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.CompanyByID(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status=%d want=%d body=%s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
}

// ---------- 409 Conflict (Replace retorna ErrDuplicateCNPJ)
func TestCompanyByID_Put_DuplicateCNPJ(t *testing.T) {
	rm := &repoMock{
		GetByIDFn: func(_ context.Context, id string) (*models.Company, error) {
			return &models.Company{ID: id, CNPJ: validCNPJ}, nil
		},
		ReplaceFn: func(_ context.Context, _ string, _ *models.Company) error {
			return repository.ErrDuplicateCNPJ
		},
	}
	h := &CompanyHandler{Repo: rm, Pub: &pubMock{}}

	body := bytes.NewBufferString(`{"cnpj":"` + validCNPJ + `","nome_fantasia":"ACME"}`)
	req := httptest.NewRequest(http.MethodPut, "/api/companies/"+companyID, body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.CompanyByID(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("status=%d want=%d body=%s", rr.Code, http.StatusConflict, rr.Body.String())
	}
}

// 5) PATCH (parcial) - go test -run 'TestCompanyByID_Patch_' -v ./internal/handlers -count=1

// ---------- 200 OK (patch válido)
func TestCompanyByID_Patch_OK(t *testing.T) {
	call := 0
	rm := &repoMock{
		GetByIDFn: func(_ context.Context, id string) (*models.Company, error) {
			call++
			if call == 1 {
				return &models.Company{ID: id, CNPJ: validCNPJ, NomeFantasia: "OLD"}, nil
			}
			// após Update, o handler busca de novo para retornar ao cliente
			return &models.Company{ID: id, CNPJ: validCNPJ, NomeFantasia: "NEW"}, nil
		},
		UpdateFn: func(_ context.Context, id string, upd *models.Company) error {
			if id != companyID {
				t.Fatalf("id inesperado: %s", id)
			}
			if upd.NomeFantasia != "NEW" {
				t.Fatalf("esperava mudar NomeFantasia para NEW; got=%q", upd.NomeFantasia)
			}
			return nil
		},
	}
	h := &CompanyHandler{Repo: rm, Pub: &pubMock{}}

	body := bytes.NewBufferString(`{"nome_fantasia":"NEW"}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/companies/"+companyID, body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.CompanyByID(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d want=%d body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	var got models.Company
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("json inválido: %v", err)
	}
	if got.NomeFantasia != "NEW" {
		t.Fatalf("payload inesperado: %#v", got)
	}
}

// ---------- 400 BAD REQUEST (JSON inválido)
func TestCompanyByID_Patch_InvalidJSON(t *testing.T) {
	h := &CompanyHandler{Repo: &repoMock{}, Pub: &pubMock{}}

	req := httptest.NewRequest(http.MethodPatch, "/api/companies/"+companyID, bytes.NewBufferString(`{`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.CompanyByID(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want=%d body=%s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
}

// ---------- 404 Not Found (primeiro GetByID não acha)
func TestCompanyByID_Patch_NotFound(t *testing.T) {
	rm := &repoMock{
		GetByIDFn: func(_ context.Context, _ string) (*models.Company, error) {
			return nil, errors.New("not found")
		},
	}
	h := &CompanyHandler{Repo: rm, Pub: &pubMock{}}

	req := httptest.NewRequest(http.MethodPatch, "/api/companies/"+companyID, bytes.NewBufferString(`{"nome_fantasia":"X"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.CompanyByID(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status=%d want=%d body=%s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
}

// ---------- 400 BAD REQUEST (CNPJ inválido no patch)
func TestCompanyByID_Patch_InvalidCNPJ(t *testing.T) {
	// precisa existir antes, senão cairia no 404
	rm := &repoMock{
		GetByIDFn: func(_ context.Context, id string) (*models.Company, error) {
			return &models.Company{ID: id, CNPJ: validCNPJ}, nil
		},
	}
	h := &CompanyHandler{Repo: rm, Pub: &pubMock{}}

	req := httptest.NewRequest(http.MethodPatch, "/api/companies/"+companyID, bytes.NewBufferString(`{"cnpj":"xx"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.CompanyByID(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want=%d body=%s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
}

// ---------- 409 Conflict (Update retorna ErrDuplicateCNPJ)
func TestCompanyByID_Patch_DuplicateCNPJ(t *testing.T) {
	rm := &repoMock{
		GetByIDFn: func(_ context.Context, id string) (*models.Company, error) {
			return &models.Company{ID: id, CNPJ: validCNPJ}, nil
		},
		UpdateFn: func(_ context.Context, _ string, _ *models.Company) error {
			return repository.ErrDuplicateCNPJ
		},
	}
	h := &CompanyHandler{Repo: rm, Pub: &pubMock{}}

	req := httptest.NewRequest(http.MethodPatch, "/api/companies/"+companyID, bytes.NewBufferString(`{"nome_fantasia":"X"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.CompanyByID(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("status=%d want=%d body=%s", rr.Code, http.StatusConflict, rr.Body.String())
	}
}

// 6) DELETE - go test -run 'TestCompanyByID_Delete_' -v ./internal/handlers -count=1

// ---------- 204 No Content (sucesso)
func TestCompanyByID_Delete_OK(t *testing.T) {
	deleted := false
	rm := &repoMock{
		GetByIDFn: func(_ context.Context, id string) (*models.Company, error) {
			return &models.Company{ID: id, NomeFantasia: "ACME"}, nil
		},
		DeleteFn: func(_ context.Context, id string) error {
			if id != companyID {
				t.Fatalf("id inesperado: %s", id)
			}
			deleted = true
			return nil
		},
	}
	h := &CompanyHandler{Repo: rm, Pub: &pubMock{}}

	req := httptest.NewRequest(http.MethodDelete, "/api/companies/"+companyID, nil)
	rr := httptest.NewRecorder()
	h.CompanyByID(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status=%d want=%d body=%s", rr.Code, http.StatusNoContent, rr.Body.String())
	}
	if !deleted {
		t.Fatal("Delete não foi chamado")
	}
}

// ---------- 404 Not Found (não existe)
func TestCompanyByID_Delete_NotFound(t *testing.T) {
	rm := &repoMock{
		GetByIDFn: func(_ context.Context, _ string) (*models.Company, error) { return nil, errors.New("not found") },
	}
	h := &CompanyHandler{Repo: rm, Pub: &pubMock{}}

	req := httptest.NewRequest(http.MethodDelete, "/api/companies/"+companyID, nil)
	rr := httptest.NewRecorder()
	h.CompanyByID(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status=%d want=%d body=%s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
}

// ---------- 500 Internal Server Error (erro ao deletar)
func TestCompanyByID_Delete_RepoError(t *testing.T) {
	rm := &repoMock{
		GetByIDFn: func(_ context.Context, id string) (*models.Company, error) {
			return &models.Company{ID: id}, nil
		},
		DeleteFn: func(_ context.Context, _ string) error { return errors.New("boom") },
	}
	h := &CompanyHandler{Repo: rm, Pub: &pubMock{}}

	req := httptest.NewRequest(http.MethodDelete, "/api/companies/"+companyID, nil)
	rr := httptest.NewRecorder()
	h.CompanyByID(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d want=%d body=%s", rr.Code, http.StatusInternalServerError, rr.Body.String())
	}
}
