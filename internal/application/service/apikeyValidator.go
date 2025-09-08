package service

import (
	"context"
	"fmt"

	"github.com/thek4n/paste.thek4n.ru/internal/application/repository"
)

// IAPIKeyService interface for APIKeyService.
type IAPIKeyService interface {
	Exists(context.Context, string) (bool, error)

	// CheckValid returns error if key not exists.
	CheckValid(context.Context, string) (bool, error)

	GetID(context.Context, string) (string, error)
}

// APIKeyService service.
type APIKeyService struct {
	repository repository.APIKeyRORepository
}

// NewAPIKeyService constructor.
func NewAPIKeyService(r repository.APIKeyRORepository) *APIKeyService {
	return &APIKeyService{
		repository: r,
	}
}

// Exists checks is apikey exists.
func (s *APIKeyService) Exists(ctx context.Context, apikey string) (bool, error) {
	exists, err := s.repository.Exists(ctx, apikey)
	if err != nil {
		return false, fmt.Errorf("fail to get key: %w", err)
	}

	return exists, nil
}

// CheckValid checks is apikey valid. Returns err if not exists.
func (s *APIKeyService) CheckValid(ctx context.Context, apikey string) (bool, error) {
	key, err := s.repository.GetByID(ctx, apikey)
	if err != nil {
		return false, fmt.Errorf("fail to get key: %w", err)
	}

	return key.Valid(), nil
}

// GetID return apikey ID. Returns err if not exists.
func (s *APIKeyService) GetID(ctx context.Context, apikey string) (string, error) {
	key, err := s.repository.GetByID(ctx, apikey)
	if err != nil {
		return "", fmt.Errorf("fail to get key: %w", err)
	}

	return key.PublicID().String(), nil
}
