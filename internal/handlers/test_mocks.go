package handlers

import (
	"context"
	"errors"

	"github.com/Werneck0live/cadastro-empresa/internal/models"

	"github.com/rabbitmq/amqp091-go"
)

type repoMock struct {
	GetAllFn  func(ctx context.Context, limit, skip int64) ([]models.Company, error)
	CreateFn  func(ctx context.Context, c *models.Company) (string, error)
	GetByIDFn func(ctx context.Context, id string) (*models.Company, error)
	UpdateFn  func(ctx context.Context, id string, upd *models.Company) error
	ReplaceFn func(ctx context.Context, id string, doc *models.Company) error
	DeleteFn  func(ctx context.Context, id string) error
}

func (m *repoMock) GetAll(ctx context.Context, limit, skip int64) ([]models.Company, error) {
	if m.GetAllFn == nil {
		return nil, errors.New("GetAllFn not set")
	}
	return m.GetAllFn(ctx, limit, skip)
}
func (m *repoMock) Create(ctx context.Context, c *models.Company) (string, error) {
	if m.CreateFn == nil {
		return "", errors.New("CreateFn not set")
	}
	return m.CreateFn(ctx, c)
}
func (m *repoMock) GetByID(ctx context.Context, id string) (*models.Company, error) {
	if m.GetByIDFn == nil {
		return nil, errors.New("GetByIDFn not set")
	}
	return m.GetByIDFn(ctx, id)
}
func (m *repoMock) Update(ctx context.Context, id string, upd *models.Company) error {
	if m.UpdateFn == nil {
		return errors.New("UpdateFn not set")
	}
	return m.UpdateFn(ctx, id, upd)
}
func (m *repoMock) Replace(ctx context.Context, id string, doc *models.Company) error {
	if m.ReplaceFn == nil {
		return errors.New("ReplaceFn not set")
	}
	return m.ReplaceFn(ctx, id, doc)
}
func (m *repoMock) Delete(ctx context.Context, id string) error {
	if m.DeleteFn == nil {
		return errors.New("DeleteFn not set")
	}
	return m.DeleteFn(ctx, id)
}

type Company struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type pubMock struct {
	PublishFn func(ctx context.Context, body string, headers amqp091.Table) error
	CloseFn   func() error
}

func (p *pubMock) Publish(ctx context.Context, body string, headers amqp091.Table) error {
	if p.PublishFn == nil {
		return nil
	}
	return p.PublishFn(ctx, body, headers)
}
func (p *pubMock) Close() error {
	if p.CloseFn == nil {
		return nil
	}
	return p.CloseFn()
}
