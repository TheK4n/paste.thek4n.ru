// Package service contains application services.
package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/thek4n/paste.thek4n.ru/internal/application/repository"
	"github.com/thek4n/paste.thek4n.ru/internal/domain/aggregate"
	"github.com/thek4n/paste.thek4n.ru/internal/domain/objectvalue"
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

// FetchAll fetch all apikeys.
func (s *APIKeysService) FetchAll() ([]aggregate.APIKey, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	apikeys, err := s.RORepository.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("fail to get apikey: %w", err)
	}

	return apikeys, nil
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

// GenerateAPIKey generates new valid APIKey.
func (s *APIKeysService) GenerateAPIKey() (aggregate.APIKey, error) {
	apikeyLength := 32
	newAPIkey, err := randomHex(apikeyLength)
	if err != nil {
		return aggregate.APIKey{}, fmt.Errorf("fail to generate api key: %w", err)
	}

	newAPIkeyID, err := uuid.NewRandom()
	if err != nil {
		return aggregate.APIKey{}, fmt.Errorf("fail to generate api key id: %w", err)
	}

	apikey := aggregate.NewAPIKey(objectvalue.APIKeyID(newAPIkeyID), newAPIkey, true)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err = s.WORepository.SetByID(ctx, newAPIkey, apikey)
	if err != nil {
		return aggregate.APIKey{}, fmt.Errorf("fail to set api key: %w", err)
	}

	return apikey, nil
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

func randomHex(n int) (string, error) {
	bytes := make([]byte, (n+1)/2)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("fail to gen random: %w", err)
	}
	hexString := hex.EncodeToString(bytes)
	return hexString[:n], nil
}
