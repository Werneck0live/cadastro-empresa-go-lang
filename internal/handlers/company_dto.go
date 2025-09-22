package handlers

//	somente os campos do contrato
//
// numero_minimo_pcd_exigidos NÃO vem do cliente (calculado no servidor)
type CompanyCreateDTO struct {
	CNPJ               string `json:"cnpj"`
	NomeFantasia       string `json:"nome_fantasia"`
	RazaoSocial        string `json:"razao_social"`
	Endereco           string `json:"endereco"`
	NumeroFuncionarios int    `json:"numero_funcionarios"`
}

// Update parcial; ponteiros distinguem "omitido" de "informado".
type CompanyPatchDTO struct {
	CNPJ               *string `json:"cnpj,omitempty"`
	NomeFantasia       *string `json:"nome_fantasia,omitempty"`
	RazaoSocial        *string `json:"razao_social,omitempty"`
	Endereco           *string `json:"endereco,omitempty"`
	NumeroFuncionarios *int    `json:"numero_funcionarios,omitempty"`
}

type CompanyPutDTO struct {
	CNPJ               *string `json:"cnpj,omitempty"`
	NomeFantasia       string  `json:"nome_fantasia"`
	RazaoSocial        string  `json:"razao_social"`
	Endereco           string  `json:"endereco"`
	NumeroFuncionarios int     `json:"numero_funcionarios"`
}
