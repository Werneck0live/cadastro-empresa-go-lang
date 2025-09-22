package admin

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/Werneck0live/cadastro-empresa/internal/models"
	"github.com/Werneck0live/cadastro-empresa/internal/repository"
	"github.com/Werneck0live/cadastro-empresa/internal/utils"
)

//go:embed seeds/companies.json
var companiesJSON []byte

type seedItem struct {
	CNPJ               string `json:"cnpj"`
	NomeFantasia       string `json:"nome_fantasia"`
	RazaoSocial        string `json:"razao_social"`
	Endereco           string `json:"endereco"`
	NumeroFuncionarios int    `json:"numero_funcionarios"`
}

// Idempotente: cria se não existir; se já existir, ignora.
func SeedCompanies(ctx context.Context, repo *repository.CompanyRepository, log *slog.Logger) error {
	var items []seedItem
	if err := json.Unmarshal(companiesJSON, &items); err != nil {
		return err
	}

	for _, s := range items {
		cnpj := utils.SanitizeCNPJ(s.CNPJ)
		if !utils.ValidateCNPJ(cnpj) {
			log.Warn("seed_skip_invalid_cnpj", "raw", s.CNPJ)
			continue
		}

		c := models.Company{
			ID:                      cnpj, // o código usa CNPJ como ID
			CNPJ:                    cnpj,
			NomeFantasia:            s.NomeFantasia,
			RazaoSocial:             s.RazaoSocial,
			Endereco:                s.Endereco,
			NumeroFuncionarios:      s.NumeroFuncionarios,
			NumeroMinimoPCDExigidos: utils.ComputeMinPCD(s.NumeroFuncionarios),
		}

		// timeout curto por item pra não travar
		ictx, cancel := context.WithTimeout(ctx, 3*time.Second)
		_, err := repo.Create(ictx, &c)
		cancel()

		if err != nil {
			if errors.Is(err, repository.ErrDuplicateCNPJ) {
				log.Info("seed_company_exists", "cnpj", cnpj)
				continue
			}
			return err
		}
		log.Info("seed_company_created", "cnpj", cnpj)
	}

	log.Info("seed_companies_done", "count", len(items))
	return nil
}
