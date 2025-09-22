package handlers

import "errors"

func validateCreateDTO(d CompanyCreateDTO) error {
	if d.CNPJ == "" {
		return errors.New("cnpj is required")
	}
	if d.NomeFantasia == "" && d.RazaoSocial == "" {
		return errors.New("either nome_fantasia or razao_social is required")
	}
	if d.NumeroFuncionarios < 0 {
		return errors.New("numero_funcionarios must be >= 0")
	}
	return nil
}

func validateUpdateDTO(d CompanyPatchDTO) error {
	if d.NumeroFuncionarios != nil && *d.NumeroFuncionarios < 0 {
		return errors.New("numero_funcionarios must be >= 0")
	}

	return nil
}

func validatePutDTO(d CompanyPutDTO) error {
	if d.NumeroFuncionarios < 0 {
		return errors.New("numero_funcionarios must be >= 0")
	}
	return nil
}
