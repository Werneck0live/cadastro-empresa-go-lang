package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Werneck0live/cadastro-empresa/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var ErrDuplicateCNPJ = errors.New("cnpj already exists")

type CompanyRepository struct {
	coll *mongo.Collection
}

func NewCompanyRepository(db *mongo.Database) *CompanyRepository {
	return &CompanyRepository{coll: db.Collection("companies")}
}

func (r *CompanyRepository) EnsureIndexes(ctx context.Context) error {
	model := mongo.IndexModel{
		Keys: bson.D{{Key: "cnpj", Value: 1}},
		Options: options.Index().
			SetUnique(true).
			SetName("uniq_cnpj"),
	}
	_, err := r.coll.Indexes().CreateOne(ctx, model)
	if err == nil {
		return nil
	}
	// Se já existir com outra opção, tenta dropar e recriar
	if ce, ok := err.(mongo.CommandError); ok && ce.Code == 85 { // IndexOptionsConflict
		if _, dropErr := r.coll.Indexes().DropOne(ctx, "uniq_cnpj"); dropErr != nil {
			return fmt.Errorf("drop index uniq_cnpj: %w", dropErr)
		}
		_, createErr := r.coll.Indexes().CreateOne(ctx, model)
		return createErr
	}
	return err
}

func (r *CompanyRepository) Create(ctx context.Context, c *models.Company) (string, error) {
	c.CreatedAt = time.Now()
	c.UpdatedAt = c.CreatedAt
	res, err := r.coll.InsertOne(ctx, c)
	if err != nil {
		if we, ok := err.(mongo.WriteException); ok {
			for _, e := range we.WriteErrors {
				if e.Code == 11000 {
					return "", ErrDuplicateCNPJ
				}
			}
		}
		return "", err
	}
	id, _ := res.InsertedID.(string) // Esse "(string)" está aqui por podemos usar "_id" como string também
	return id, nil
}

func (r *CompanyRepository) GetByID(ctx context.Context, id string) (*models.Company, error) {
	var c models.Company
	err := r.coll.FindOne(ctx, bson.M{"_id": id}).Decode(&c)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *CompanyRepository) GetAll(ctx context.Context, limit int64, skip int64) ([]models.Company, error) {
	opts := options.Find().SetLimit(limit).SetSkip(skip).SetSort(bson.D{{Key: "created_at", Value: -1}})
	cur, err := r.coll.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	list := []models.Company{}
	for cur.Next(ctx) {
		var c models.Company
		if err := cur.Decode(&c); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, cur.Err()
}

func (r *CompanyRepository) Update(ctx context.Context, id string, c *models.Company) error {
	now := time.Now()
	set := bson.M{
		"updated_at": now,
	}

	if c.NomeFantasia != "" {
		set["nome_fantasia"] = c.NomeFantasia
	}
	if c.RazaoSocial != "" {
		set["razao_social"] = c.RazaoSocial
	}
	if c.Endereco != "" {
		set["endereco"] = c.Endereco
	}
	if c.NumeroFuncionarios != 0 {
		set["numero_funcionarios"] = c.NumeroFuncionarios
	}
	if c.NumeroMinimoPCDExigidos != 0 {
		set["numero_minimo_pcd_exigidos"] = c.NumeroMinimoPCDExigidos
	}
	if c.CNPJ != "" {
		set["cnpj"] = c.CNPJ
	}

	_, err := r.coll.UpdateByID(ctx, id, bson.M{"$set": set})
	if err != nil {
		if we, ok := err.(mongo.WriteException); ok {
			for _, e := range we.WriteErrors {
				if e.Code == 11000 {
					return ErrDuplicateCNPJ
				}
			}
		}
	}
	return err
}

func (r *CompanyRepository) Replace(ctx context.Context, id string, c *models.Company) error {
	filter := bson.M{"_id": id}
	_, err := r.coll.ReplaceOne(ctx, filter, c)
	if err != nil {
		if we, ok := err.(mongo.WriteException); ok {
			for _, e := range we.WriteErrors {
				if e.Code == 11000 {
					return ErrDuplicateCNPJ
				}
			}
		}
	}
	return err
}

func (r *CompanyRepository) Delete(ctx context.Context, id string) error {
	_, err := r.coll.DeleteOne(ctx, bson.M{"_id": id})
	return err
}
