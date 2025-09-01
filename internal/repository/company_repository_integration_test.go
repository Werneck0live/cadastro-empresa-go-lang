//go:build integration
// +build integration

package repository

import (
	"context"
	"testing"
	"time"

	/*
		Para Rodar: go test -tags=integration -v ./internal/repository -run TestCompanyRepository_Integration -count=1

		obs: Rodar todos os de integração: go test -tags=integration -v ./... -count=1
	*/

	"github.com/Werneck0live/cadastro-empresa/internal/db"
	"github.com/Werneck0live/cadastro-empresa/internal/models"
	"github.com/Werneck0live/cadastro-empresa/internal/utils"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
)

// Testa os métodos: Create -> GetByID -> Update -> Replace -> Delete
func TestCompanyRepository_Integration_CreateGetUpdateReplaceDelete(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Sobe Mongo real
	mongoC, err := mongodb.RunContainer(ctx, tc.WithImage("mongo:7"))
	if err != nil {
		t.Fatalf("start mongo: %v", err)
	}
	t.Cleanup(func() { _ = mongoC.Terminate(ctx) })

	uri, err := mongoC.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("conn string: %v", err)
	}

	// Conecta com o helper
	client, err := db.NewMongoClient(uri)
	if err != nil {
		t.Fatalf("mongo client: %v", err)
	}
	t.Cleanup(func() { _ = client.Disconnect(ctx) })

	database := client.Database("testdb")
	repo := NewCompanyRepository(database) // do pacote do repository

	// 1) Create
	now := time.Now().UTC()
	c := models.Company{
		ID:                 "11222333000181", // use ID=CNPJ sanitizado, se é sua regra
		CNPJ:               "11.222.333/0001-81",
		NomeFantasia:       "ACME",
		RazaoSocial:        "ACME S.A.",
		Endereco:           "Rua X, 123",
		NumeroFuncionarios: 50,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	id, err := repo.Create(ctx, &c)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if id == "" {
		t.Fatalf("create: id vazio")
	}

	// 2) GetByID
	got, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if got == nil || got.NomeFantasia != "ACME" {
		t.Fatalf("get mismatch: %#v", got)
	}

	if got.NumeroMinimoPCDExigidos != utils.ComputeMinPCD(got.NumeroMinimoPCDExigidos) {
		t.Fatalf("fail calc pcd (create-method): got=%d", got.NumeroMinimoPCDExigidos)
	}

	// 3) Update (patch - parcial)
	err = repo.Update(ctx, id, &models.Company{NomeFantasia: "ACME NEW"})
	if err != nil {
		t.Fatalf("update 'NomeFantasia': %v", err)
	}
	got2, err := repo.GetByID(ctx, id)
	if err != nil || got2 == nil || got2.NomeFantasia != "ACME NEW" {
		t.Fatalf("after update mismatch: %#v err=%v", got2, err)
	}

	// ComputeMinPCD
	err = repo.Update(ctx, id, &models.Company{NumeroFuncionarios: 102})
	if err != nil {
		t.Fatalf("update 'NumeroFuncionarios': %v", err)
	}
	got3, err := repo.GetByID(ctx, id)
	if got3.NumeroMinimoPCDExigidos != utils.ComputeMinPCD(got3.NumeroMinimoPCDExigidos) {
		t.Fatalf("fail calc pcd (update-method): got=%d", got3.NumeroMinimoPCDExigidos)
	}

	// 4) Replace (PUT)
	newDoc := models.Company{
		ID:                 id,
		CNPJ:               "11.222.333/0001-81",
		NomeFantasia:       "ACME REPLACED",
		RazaoSocial:        "ACME S.A.",
		Endereco:           "Rua Y, 456",
		NumeroFuncionarios: 210,
		CreatedAt:          got.CreatedAt, // preserve
		UpdatedAt:          time.Now().UTC(),
	}
	if err := repo.Replace(ctx, id, &newDoc); err != nil {
		t.Fatalf("replace: %v", err)
	}

	got4, err := repo.GetByID(ctx, id)
	if err != nil || got4 == nil || got4.NomeFantasia != "ACME REPLACED" {
		t.Fatalf("after replace mismatch: %#v err=%v", got4, err)
	}

	if got4.NumeroMinimoPCDExigidos != utils.ComputeMinPCD(got4.NumeroMinimoPCDExigidos) {
		t.Fatalf("fail calc pcd (put-method): got=%d", got4.NumeroMinimoPCDExigidos)
	}

	// 5) Delete
	if err := repo.Delete(ctx, id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.GetByID(ctx, id); err == nil {
		t.Fatalf("expected not found after delete")
	}
}
