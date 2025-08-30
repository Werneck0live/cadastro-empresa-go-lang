package models

import "time"

type Company struct {
	ID                          string    `bson:"_id,omitempty" json:"id"`
	CNPJ                        string    `bson:"cnpj" json:"cnpj"` // armazenado normalizado (apenas dígitos)
	NomeFantasia                string    `bson:"nome_fantasia" json:"nome_fantasia"`
	RazaoSocial                 string    `bson:"razao_social" json:"razao_social"`
	Endereco                    string    `bson:"endereco" json:"endereco"`
	NumeroFuncionarios      	int       `json:"numero_funcionarios" bson:"numero_funcionarios"`
	NumeroMinimoPCDExigidos 	int       `json:"numero_minimo_pcd_exigidos" bson:"numero_minimo_pcd_exigidos"`
	CreatedAt                   time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt                   time.Time `bson:"updated_at" json:"updated_at"`
}
