// Package service contains domain services.
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/thek4n/paste.thek4n.ru/internal/domain/repository"
)

// APIKeysService provides methods to work with apikeys.
type APIKeysService struct {
	RORepository repository.APIKeyRORepository
	WORepository repository.APIKeyWORepository
}

// NewAPIKeysService constructor.
func NewAPIKeysService(
	getRep repository.APIKeyRORepository,
	setRep repository.APIKeyWORepository,
) *APIKeysService {
	return &APIKeysService{
		RORepository: getRep,
		WORepository: setRep,
	}
}

// InvalidateAPIKey invalidates apikey by id.
func (s *APIKeysService) InvalidateAPIKey(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	apikey, err := s.RORepository.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("fail to get apikey: %w", err)
	}

	apikey.Invalidate()

	if err := s.WORepository.SetByID(ctx, id, apikey); err != nil {
		return fmt.Errorf("fail to set apikey: %w", err)
	}

	return nil
}

// ReauthorizeAPIKey reauthorizes apikey by id.
func (s *APIKeysService) ReauthorizeAPIKey(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	apikey, err := s.RORepository.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("fail to get apikey: %w", err)
	}

	apikey.Reauthorize()

	if err := s.WORepository.SetByID(ctx, id, apikey); err != nil {
		return fmt.Errorf("fail to set apikey: %w", err)
	}

	return nil
}

// RemoveAPIKey removes apikey by id.
func (s *APIKeysService) RemoveAPIKey(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := s.WORepository.RemoveByID(ctx, id); err != nil {
		return fmt.Errorf("fail to remove apikey: %w", err)
	}
	return nil
}
