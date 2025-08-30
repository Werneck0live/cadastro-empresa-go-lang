package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rabbitmq/amqp091-go"
	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/Werneck0live/cadastro-empresa/internal/models"
	"github.com/Werneck0live/cadastro-empresa/internal/repository"
	"github.com/Werneck0live/cadastro-empresa/internal/utils"
)

type Repository interface {
	GetAll(ctx context.Context, limit, skip int64) ([]models.Company, error)
	Create(ctx context.Context, c *models.Company) (string, error)
	GetByID(ctx context.Context, id string) (*models.Company, error)
	Update(ctx context.Context, id string, upd *models.Company) error
	Replace(ctx context.Context, id string, doc *models.Company) error
	Delete(ctx context.Context, id string) error
}

// type Publisher interface {
// 	Publish(ctx context.Context, payload []byte) error
// 	Close() error
// }

type Publisher interface {
	Publish(ctx context.Context, body string, headers amqp091.Table) error
	Close() error
}

type CompanyHandler struct {
	Repo Repository
	Pub  Publisher
}

func NewCompanyHandler(repo Repository, pub Publisher) *CompanyHandler {
	return &CompanyHandler{Repo: repo, Pub: pub}
}

// garantir que a requisição venha no padrão /api/companies/{id_company}
func parseIDFromPath(path string) (string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 3 && parts[0] == "api" && parts[1] == "companies" && parts[2] != "" {
		return parts[2], true
	}
	return "", false
}

func (h *CompanyHandler) Health(w http.ResponseWriter, r *http.Request) {
	utils.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})

}

func (h *CompanyHandler) Companies(w http.ResponseWriter, r *http.Request) {

	switch r.Method {

	// "getAll", "getAll-pagination"(skip, limit)
	case http.MethodGet:
		q := r.URL.Query()
		limit := int64(50)
		skip := int64(0)
		if l := q.Get("limit"); l != "" {
			if v, err := strconv.ParseInt(l, 10, 64); err == nil && v > 0 && v <= 200 {
				limit = v
			}
		}
		if s := q.Get("skip"); s != "" {
			if v, err := strconv.ParseInt(s, 10, 64); err == nil && v >= 0 {
				skip = v
			}
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		list, err := h.Repo.GetAll(ctx, limit, skip)
		if err != nil {
			utils.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		utils.WriteJSON(w, http.StatusOK, list)

	// create
	case http.MethodPost:
		var dto CompanyCreateDTO
		if err := utils.DecodeStrict(r.Body, &dto); err != nil {
			utils.BadRequest(w, utils.FormatUnknownFieldError(err))
			return
		}
		if err := validateCreateDTO(dto); err != nil {
			utils.BadRequest(w, err.Error())
			return
		}

		c := models.Company{
			CNPJ:               utils.SanitizeCNPJ(dto.CNPJ),
			NomeFantasia:       dto.NomeFantasia,
			RazaoSocial:        dto.RazaoSocial,
			Endereco:           dto.Endereco,
			NumeroFuncionarios: dto.NumeroFuncionarios,
		}
		if !utils.ValidateCNPJ(c.CNPJ) {
			utils.BadRequest(w, "invalid cnpj")
			return
		}
		c.ID = c.CNPJ

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		if _, err := h.Repo.Create(ctx, &c); err != nil {
			if errors.Is(err, repository.ErrDuplicateCNPJ) {
				utils.WriteJSON(w, http.StatusConflict, map[string]string{"error": "cnpj already exists"})
				return
			}
			utils.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		h.publishEvent("Cadastro", &c)
		utils.WriteJSON(w, http.StatusCreated, c)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (h *CompanyHandler) CompanyByID(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDFromPath(r.URL.Path)
	if !ok {
		utils.WriteJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}

	// log.Printf("Método usado: ", r.Method)

	switch r.Method {
	case http.MethodGet:
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		c, err := h.Repo.GetByID(ctx, id)
		if err != nil {
			utils.WriteJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		utils.WriteJSON(w, http.StatusOK, c)

	case http.MethodPatch:
		var dto CompanyPatchDTO
		if err := utils.DecodeStrict(r.Body, &dto); err != nil {
			utils.BadRequest(w, utils.FormatUnknownFieldError(err))
			return
		}

		if err := validateUpdateDTO(dto); err != nil {
			utils.BadRequest(w, err.Error())
			return
		}

		// Buscar atual para validar CNPJ e (opcional) recalcular PCD
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		existing, err := h.Repo.GetByID(ctx, id)
		if err != nil {
			utils.WriteJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}

		// Monta o modelo para update apenas com campos presentes
		upd := models.Company{}

		if dto.CNPJ != nil {
			cnpj := utils.SanitizeCNPJ(*dto.CNPJ)
			if !utils.ValidateCNPJ(cnpj) {
				utils.BadRequest(w, "invalid cnpj")
				return
			}
			// Só tente mudar se for diferente do atual
			if cnpj != existing.CNPJ {
				upd.CNPJ = cnpj
			}
		}
		if dto.NomeFantasia != nil {
			upd.NomeFantasia = *dto.NomeFantasia
		}
		if dto.RazaoSocial != nil {
			upd.RazaoSocial = *dto.RazaoSocial
		}
		if dto.Endereco != nil {
			upd.Endereco = *dto.Endereco
		}

		if dto.NumeroFuncionarios != nil {
			upd.NumeroFuncionarios = *dto.NumeroFuncionarios

			upd.NumeroMinimoPCDExigidos = utils.ComputeMinPCD(upd.NumeroFuncionarios)
		}

		if err := h.Repo.Update(ctx, id, &upd); err != nil {
			if errors.Is(err, repository.ErrDuplicateCNPJ) {
				utils.WriteJSON(w, http.StatusConflict, map[string]string{"error": "cnpj already exists"})
				return
			}
			utils.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		// Retorna o doc atualizado
		c2, _ := h.Repo.GetByID(ctx, id)
		if c2 != nil {
			h.publishEvent("Edição", c2)
			utils.WriteJSON(w, http.StatusOK, c2)
			return
		}
		utils.WriteJSON(w, http.StatusOK, map[string]string{"id": id})

	case http.MethodPut:
		var dto CompanyPutDTO
		if err := utils.DecodeStrict(r.Body, &dto); err != nil {
			utils.BadRequest(w, utils.FormatUnknownFieldError(err))
			return
		}
		if err := validatePutDTO(dto); err != nil {
			// log.Println("\n\n\n\n\n\n\n")
			// log.Println("%v", err)
			// log.Println("Teste")
			utils.BadRequest(w, err.Error())
			return
		}

		// {id} da rota
		id, ok := parseIDFromPath(r.URL.Path)
		if !ok {
			utils.WriteJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		current, err := h.Repo.GetByID(ctx, id)
		if err != nil {
			utils.WriteJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}

		// Regras para CNPJ:
		// - se não vier no body, usar o {id}
		// - se vier, deve ser igual ao {id}
		var cnpj string
		if dto.CNPJ == nil {
			cnpj = id
		} else {
			cnpj = utils.SanitizeCNPJ(*dto.CNPJ)
			if cnpj != id {
				utils.BadRequest(w, "cnpj in body must match the resource id in path")
				return
			}
		}
		if !utils.ValidateCNPJ(cnpj) {
			utils.BadRequest(w, "invalid cnpj")
			return
		}

		// monta o documento COMPLETO que substituirá o atual (PUT = replace)
		newDoc := models.Company{
			ID:                      id, // preserva o mesmo _id
			CNPJ:                    cnpj,
			NomeFantasia:            dto.NomeFantasia,
			RazaoSocial:             dto.RazaoSocial,
			Endereco:                dto.Endereco,
			NumeroFuncionarios:      dto.NumeroFuncionarios,
			NumeroMinimoPCDExigidos: utils.ComputeMinPCD(dto.NumeroFuncionarios), // se você estiver usando compute
			CreatedAt:               current.CreatedAt,                           // preserva criação
			UpdatedAt:               time.Now(),
		}

		if err := h.Repo.Replace(ctx, id, &newDoc); err != nil {
			if errors.Is(err, repository.ErrDuplicateCNPJ) {
				utils.WriteJSON(w, http.StatusConflict, map[string]string{"error": "cnpj already exists"})
				return
			}
			utils.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		h.publishEvent("Edição", &newDoc)
		utils.WriteJSON(w, http.StatusOK, newDoc)

	case http.MethodDelete:
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		// Busca antes de deletar para logar o nome
		c, err := h.Repo.GetByID(ctx, id)
		if err != nil {
			utils.WriteJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}

		if err := h.Repo.Delete(ctx, id); err != nil {
			utils.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		h.publishEvent("Exclusão", c)
		w.WriteHeader(http.StatusNoContent)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (h *CompanyHandler) publishEvent(acao string, c *models.Company) {
	if h.Pub == nil || c == nil {
		return
	}
	// Escolhe o nome a exibir
	empresa := c.NomeFantasia
	if empresa == "" {
		if c.RazaoSocial != "" {
			empresa = c.RazaoSocial
		} else {
			empresa = c.CNPJ
		}
	}
	msg := fmt.Sprintf("%s de EMPRESA %s", acao, empresa)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_ = h.Pub.Publish(ctx, msg, amqp.Table{
		"action":     strings.ToLower(acao), // cadastro|edição|exclusão
		"company_id": c.ID,
		"cnpj":       c.CNPJ,
		"nome":       empresa,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	})
}
