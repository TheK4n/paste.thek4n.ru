package service

import (
	"context"
	"fmt"
	"time"

	"github.com/thek4n/paste.thek4n.ru/internal/domain/aggregate"
	"github.com/thek4n/paste.thek4n.ru/internal/domain/objectvalue"
	"github.com/thek4n/paste.thek4n.ru/internal/domain/repository"
)

// GetService domain service for getting records.
type GetService struct {
	recordRepository repository.RecordRepository
}

// NewGetService constructor.
func NewGetService(recordRepository repository.RecordRepository) *GetService {
	return &GetService{
		recordRepository: recordRepository,
	}
}

// GetBodyAnswer GetService result.
type GetBodyAnswer struct {
	Body  []byte
	IsURL bool
}

// GetBody returns GetBodyAnswer. If not exists returns ErrRecordNotFound as error.
func (h *GetService) GetBody(key objectvalue.RecordKey) (GetBodyAnswer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	record, err := h.get(ctx, key)
	if err != nil {
		return GetBodyAnswer{}, err
	}

	body, err := record.GetBody()
	if err != nil {
		return GetBodyAnswer{}, fmt.Errorf("fail to read record body: %w", err)
	}

	err = h.recordRepository.SetByKey(ctx, key, record)
	if err != nil {
		return GetBodyAnswer{}, fmt.Errorf("fail to write record: %w", err)
	}

	return GetBodyAnswer{
		Body:  body,
		IsURL: record.URL(),
	}, nil
}

// GetClicks returns GetBodyAnswer. If not exists returns ErrRecordNotFound as error.
func (h *GetService) GetClicks(key objectvalue.RecordKey) (uint32, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	record, err := h.get(ctx, key)
	if err != nil {
		return 0, err
	}

	return record.Clicks(), nil
}

func (h *GetService) get(ctx context.Context, key objectvalue.RecordKey) (aggregate.Record, error) {
	record, err := h.recordRepository.GetByKey(ctx, key)
	if err != nil {
		return aggregate.Record{}, fmt.Errorf("fail to get record from repository: %w", err)
	}

	return record, nil
}
