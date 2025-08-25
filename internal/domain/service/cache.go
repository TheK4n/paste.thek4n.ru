package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/thek4n/paste.thek4n.ru/internal/domain/aggregate"
	"github.com/thek4n/paste.thek4n.ru/internal/domain/config"
	"github.com/thek4n/paste.thek4n.ru/internal/domain/domainerrors"
	"github.com/thek4n/paste.thek4n.ru/internal/domain/event"
	"github.com/thek4n/paste.thek4n.ru/internal/domain/objectvalue"
	"github.com/thek4n/paste.thek4n.ru/internal/domain/repository"
	"github.com/thek4n/paste.thek4n.ru/pkg/apikeys"
)

// CacheService domain service.
type CacheService struct {
	recordRepository repository.RecordRepository
	quotaRepository  repository.QuotaRepository
	apikeyRepository repository.APIKeyRORepository
	eventPublisher   *event.Publisher
	validationConfig config.CacheValidationConfig
	quotaConfig      config.QuotaConfig
}

// NewCacheService constructor.
func NewCacheService(
	recordRepository repository.RecordRepository,
	quotaRepository repository.QuotaRepository,
	apikeyRepository repository.APIKeyRORepository,
	eventPublisher *event.Publisher,
	cfg config.CacheValidationConfig,
	quotacfg config.QuotaConfig,
) *CacheService {
	return &CacheService{
		recordRepository: recordRepository,
		quotaRepository:  quotaRepository,
		apikeyRepository: apikeyRepository,
		eventPublisher:   eventPublisher,
		validationConfig: cfg,
		quotaConfig:      quotacfg,
	}
}

// Serve service method that serve cache request.
func (h *CacheService) Serve(params objectvalue.CacheRequestParams) (objectvalue.RecordKey, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	privileged := false
	apikeyID := ""

	if params.APIKey != "" {
		apikey, err := h.apikeyRepository.GetByID(ctx, params.APIKey)
		if err != nil {
			return objectvalue.RecordKey(""), err
		}
		privileged = apikey.Valid()
		apikeyID = apikey.PublicID().String()
	}

	if privileged {
		err := h.validatePrivielegedRequestParams(params)
		if err != nil {
			return objectvalue.RecordKey(""), err
		}

		h.logAPIKeyUsage(apikeyID, params)
		return h.servePrivileged(ctx, params)
	}

	err := h.validateUnprivilegedRequestParams(params)
	if err != nil {
		return objectvalue.RecordKey(""), err
	}

	return h.serveUnprivileged(ctx, params)
}

func (h *CacheService) servePrivileged(ctx context.Context, params objectvalue.CacheRequestParams) (objectvalue.RecordKey, error) {
	newRecord, err := aggregate.NewRecord(
		"",
		time.Now().Add(params.TTL),
		params.Disposable,
		params.Disposable == 0,
		0,
		params.Body,
		params.IsURL,
	)
	if err != nil {
		return objectvalue.RecordKey(""), err
	}

	newRecordKey, err := h.getRecordKey(ctx, params)
	if err != nil {
		return newRecordKey, err
	}

	if err := h.recordRepository.SetByKey(ctx, newRecordKey, newRecord); err != nil {
		return newRecordKey, fmt.Errorf("fail to set new record: %w", err)
	}

	return newRecordKey, nil
}

func (h *CacheService) getRecordKey(ctx context.Context, params objectvalue.CacheRequestParams) (objectvalue.RecordKey, error) {
	if params.RequestedKey != "" {
		requestedRecordKeyExists, err := h.recordRepository.Exists(ctx, objectvalue.RecordKey(params.RequestedKey))
		if err != nil {
			return objectvalue.RecordKey(""), fmt.Errorf("fail to check requested key existing: %w", err)
		}

		if requestedRecordKeyExists {
			return objectvalue.RecordKey(""), domainerrors.ErrRequestedKeyExists
		}

		return objectvalue.RecordKey(params.RequestedKey), nil
	}

	keyLength := h.validationConfig.DefaultKeyLength()
	if params.RequestedKeyLength != 0 {
		keyLength = params.RequestedKeyLength
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	newRecordKey, err := h.recordRepository.GenerateUniqueKey(ctx, keyLength, h.validationConfig.MaxKeyLength())
	if err != nil {
		return newRecordKey, fmt.Errorf("fail to generate unique key: %w", err)
	}

	return newRecordKey, nil
}

func (h *CacheService) serveUnprivileged(ctx context.Context, params objectvalue.CacheRequestParams) (objectvalue.RecordKey, error) {
	newRecord, err := aggregate.NewRecord(
		"",
		time.Now().Add(params.TTL),
		params.Disposable,
		params.Disposable == 0,
		0,
		params.Body,
		params.IsURL,
	)
	if err != nil {
		return objectvalue.RecordKey(""), err
	}

	var newRecordKey objectvalue.RecordKey

	keyLength := h.validationConfig.DefaultKeyLength()
	if params.RequestedKeyLength != 0 {
		keyLength = params.RequestedKeyLength
	}

	newRecordKey, err = h.recordRepository.GenerateUniqueKey(ctx, keyLength, h.validationConfig.MaxKeyLength())
	if err != nil {
		return newRecordKey, fmt.Errorf("fail to generate unique key: %w", err)
	}

	if err := h.recordRepository.SetByKey(ctx, newRecordKey, newRecord); err != nil {
		return newRecordKey, fmt.Errorf("fail to write record: %w", err)
	}

	err = h.manageQuota(ctx, objectvalue.QuotaSourceIP(params.SourceIP))
	if err != nil {
		return newRecordKey, err
	}

	return newRecordKey, nil
}

func (h *CacheService) manageQuota(ctx context.Context, sourceIP objectvalue.QuotaSourceIP) error {
	quota, err := h.quotaRepository.GetByID(ctx, sourceIP)
	if errors.Is(err, domainerrors.ErrQuotaNotFound) {
		quota, err = aggregate.NewQuota(sourceIP, h.quotaConfig.Quota())
		if err != nil {
			return fmt.Errorf("fail to create new quota: %w", err)
		}
	}
	if err != nil {
		return fmt.Errorf("fail to get quota: %w", err)
	}

	quota.Sub()
	if quota.Exhausted() {
		return domainerrors.ErrQuotaExhausted
	}
	err = h.quotaRepository.SetByID(ctx, sourceIP, quota)
	if err != nil {
		return fmt.Errorf("fail to write quota: %w", err)
	}

	return nil
}

func (h *CacheService) validatePrivielegedRequestParams(params objectvalue.CacheRequestParams) error {
	if params.RequestedKey != "" {
		if len(params.RequestedKey) > int(h.validationConfig.MaxKeyLength()) {
			return domainerrors.ErrInvalidRequestedKey
		}

		if len(params.RequestedKey) < int(h.validationConfig.PrivilegedMinKeyLength()) {
			return domainerrors.ErrInvalidRequestedKey
		}

		for _, char := range params.RequestedKey {
			if !strings.ContainsRune(h.validationConfig.AllowedKeyChars(), char) {
				return fmt.Errorf("%w: requested key contains illegal char", domainerrors.ErrInvalidRequestedKey)
			}
		}
	}

	if params.BodyLen > h.validationConfig.PrivilegedMaxBodySize() {
		return domainerrors.ErrBodyTooLarge
	}

	if params.TTL > h.validationConfig.PrivilegedMaxTTL() {
		return domainerrors.ErrInvalidTTL
	}

	if params.RequestedKeyLength < h.validationConfig.PrivilegedMinKeyLength() {
		return domainerrors.ErrInvalidRequestedKeyLength
	}

	return h.validateCommonRequestParams(params)
}

func (h *CacheService) validateUnprivilegedRequestParams(params objectvalue.CacheRequestParams) error {
	if params.RequestedKey != "" {
		return domainerrors.ErrNonAuthorized
	}

	if params.BodyLen > h.validationConfig.UnprivilegedMaxBodySize() {
		return domainerrors.ErrBodyTooLarge
	}

	if params.TTL < h.validationConfig.MinTTL() {
		return domainerrors.ErrInvalidTTL
	}

	if params.TTL > h.validationConfig.UnprivilegedMaxTTL() {
		return domainerrors.ErrInvalidTTL
	}

	if params.RequestedKeyLength < h.validationConfig.UnprivilegedMinKeyLength() {
		return domainerrors.ErrInvalidRequestedKeyLength
	}

	return h.validateCommonRequestParams(params)
}

func (h *CacheService) validateCommonRequestParams(params objectvalue.CacheRequestParams) error {
	if params.RequestedKeyLength > h.validationConfig.MaxKeyLength() {
		return domainerrors.ErrInvalidRequestedKeyLength
	}

	return nil
}

func (h *CacheService) logAPIKeyUsage(apikeyID string, params objectvalue.CacheRequestParams) {
	var reason apikeys.UsageReason

	switch {
	case params.TTL == 0:
		reason = apikeys.UsageReason_PERSISTKEY
	case params.RequestedKeyLength < h.validationConfig.UnprivilegedMinKeyLength():
		reason = apikeys.UsageReason_CUSTOMKEYLEN
	case params.RequestedKey != "":
		reason = apikeys.UsageReason_CUSTOMKEY
	case params.BodyLen > h.validationConfig.UnprivilegedMaxBodySize():
		reason = apikeys.UsageReason_LARGEBODY
	default:
		return
	}

	event := event.NewAPIKeyUsedEvent(apikeyID, reason, params.SourceIP)
	h.eventPublisher.NotifyAll(event)
}
