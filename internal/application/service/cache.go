package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/thek4n/paste.thek4n.ru/internal/application/repository"
	"github.com/thek4n/paste.thek4n.ru/internal/domain/aggregate"
	"github.com/thek4n/paste.thek4n.ru/internal/domain/config"
	"github.com/thek4n/paste.thek4n.ru/internal/domain/domainerrors"
	"github.com/thek4n/paste.thek4n.ru/internal/domain/event"
	"github.com/thek4n/paste.thek4n.ru/internal/domain/logger"
	"github.com/thek4n/paste.thek4n.ru/internal/domain/objectvalue"
	"github.com/thek4n/paste.thek4n.ru/pkg/apikeys"
)

// CacheService application service.
type CacheService struct {
	recordRepository repository.RecordRepository
	quotaRepository  repository.QuotaRepository
	apikeyRepository repository.APIKeyRORepository
	eventPublisher   *event.Publisher
	validationConfig config.CacheValidationConfig
	quotaConfig      config.QuotaConfig
	logger           logger.Logger
}

// NewCacheService constructor.
func NewCacheService(
	recordRepository repository.RecordRepository,
	quotaRepository repository.QuotaRepository,
	apikeyRepository repository.APIKeyRORepository,
	eventPublisher *event.Publisher,
	cfg config.CacheValidationConfig,
	quotacfg config.QuotaConfig,
	lgr logger.Logger,
) *CacheService {
	return &CacheService{
		recordRepository: recordRepository,
		quotaRepository:  quotaRepository,
		apikeyRepository: apikeyRepository,
		eventPublisher:   eventPublisher,
		validationConfig: cfg,
		quotaConfig:      quotacfg,
		logger:           lgr,
	}
}

// Serve service method that serve cache request.
func (s *CacheService) Serve(params objectvalue.CacheRequestParams) (objectvalue.RecordKey, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	privileged := false
	apikeyID := ""

	if params.APIKey != "" {
		apikey, err := s.apikeyRepository.GetByID(ctx, params.APIKey)
		if err != nil {
			return objectvalue.RecordKey(""), err
		}
		privileged = apikey.Valid()

		if !apikey.Valid() {
			s.logger.Warn("Using invalid apikey", "apikey", apikeyID)
			return objectvalue.RecordKey(""), domainerrors.ErrAPIKeyInvalid
		}

		apikeyID = apikey.PublicID().String()
	}

	if privileged {
		s.logger.Info("Authorize APIKey", "apikey", apikeyID)
	}

	if privileged {
		err := s.validatePrivielegedRequestParams(params)
		if err != nil {
			return objectvalue.RecordKey(""), err
		}

		s.logAPIKeyUsage(apikeyID, params)
		return s.servePrivileged(ctx, params)
	}

	err := s.validateUnprivilegedRequestParams(params)
	if err != nil {
		return objectvalue.RecordKey(""), err
	}

	return s.serveUnprivileged(ctx, params)
}

func (s *CacheService) servePrivileged(ctx context.Context, params objectvalue.CacheRequestParams) (objectvalue.RecordKey, error) {
	expirationDate := objectvalue.NewExpirationDateFromTTL(params.TTL)
	newRecord := aggregate.NewRecord(
		"",
		expirationDate,
		params.Disposable,
		params.Disposable == 0,
		0,
		params.Body,
		params.IsURL,
	)

	newRecordKey, err := s.getRecordKey(ctx, params)
	if err != nil {
		return newRecordKey, err
	}

	if err := s.recordRepository.SetByKey(ctx, newRecordKey, newRecord); err != nil {
		return newRecordKey, fmt.Errorf("fail to set new record: %w", err)
	}

	return newRecordKey, nil
}

func (s *CacheService) getRecordKey(ctx context.Context, params objectvalue.CacheRequestParams) (objectvalue.RecordKey, error) {
	if params.RequestedKey != "" {
		requestedRecordKeyExists, err := s.recordRepository.Exists(ctx, objectvalue.RecordKey(params.RequestedKey))
		if err != nil {
			return objectvalue.RecordKey(""), fmt.Errorf("fail to check requested key existing: %w", err)
		}

		if requestedRecordKeyExists {
			return objectvalue.RecordKey(""), domainerrors.ErrRequestedKeyExists
		}

		return objectvalue.RecordKey(params.RequestedKey), nil
	}

	keyLength := s.validationConfig.DefaultKeyLength()
	if params.RequestedKeyLength != 0 {
		keyLength = params.RequestedKeyLength
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	newRecordKey, err := s.recordRepository.GenerateUniqueKey(ctx, keyLength, s.validationConfig.MaxKeyLength())
	if err != nil {
		return newRecordKey, fmt.Errorf("fail to generate unique key: %w", err)
	}

	return newRecordKey, nil
}

func (s *CacheService) serveUnprivileged(ctx context.Context, params objectvalue.CacheRequestParams) (objectvalue.RecordKey, error) {
	expirationDate := objectvalue.NewExpirationDateFromTTL(params.TTL)
	newRecord := aggregate.NewRecord(
		"",
		expirationDate,
		params.Disposable,
		params.Disposable == 0,
		0,
		params.Body,
		params.IsURL,
	)

	var newRecordKey objectvalue.RecordKey

	keyLength := s.validationConfig.DefaultKeyLength()
	if params.RequestedKeyLength != 0 {
		keyLength = params.RequestedKeyLength
	}

	newRecordKey, err := s.recordRepository.GenerateUniqueKey(ctx, keyLength, s.validationConfig.MaxKeyLength())
	if err != nil {
		return newRecordKey, fmt.Errorf("fail to generate unique key: %w", err)
	}

	if err := s.recordRepository.SetByKey(ctx, newRecordKey, newRecord); err != nil {
		return newRecordKey, fmt.Errorf("fail to write record: %w", err)
	}

	err = s.manageQuota(ctx, objectvalue.QuotaSourceIP(params.SourceIP))
	if err != nil {
		return newRecordKey, err
	}

	return newRecordKey, nil
}

func (s *CacheService) manageQuota(ctx context.Context, sourceIP objectvalue.QuotaSourceIP) error {
	quota, err := s.quotaRepository.GetByID(ctx, sourceIP)
	if err != nil {
		if errors.Is(err, domainerrors.ErrQuotaNotFound) {
			quota = aggregate.NewQuota(sourceIP, s.quotaConfig.Quota())
		} else {
			s.logger.Error("Fail to get quota", "error", err.Error(), "source_ip", string(sourceIP))
			return fmt.Errorf("fail to get quota: %w", err)
		}
	}

	quota.Sub()
	s.logger.Info("Sub quota", "source_ip", string(sourceIP), "quota", quota.Value())
	if quota.Exhausted() {
		return domainerrors.ErrQuotaExhausted
	}
	err = s.quotaRepository.SetByID(ctx, sourceIP, quota)
	if err != nil {
		s.logger.Warn("Fail to set quota", "error", err.Error(), "source_ip", string(sourceIP))
		return fmt.Errorf("fail to write quota: %w", err)
	}

	return nil
}

func (s *CacheService) validatePrivielegedRequestParams(params objectvalue.CacheRequestParams) error {
	if params.RequestedKey != "" {
		if len(params.RequestedKey) > int(s.validationConfig.MaxKeyLength()) {
			return domainerrors.ErrInvalidRequestedKey
		}

		if len(params.RequestedKey) < int(s.validationConfig.PrivilegedMinKeyLength()) {
			return domainerrors.ErrInvalidRequestedKey
		}

		for _, char := range params.RequestedKey {
			if !strings.ContainsRune(s.validationConfig.AllowedKeyChars(), char) {
				return fmt.Errorf("%w: requested key contains illegal char", domainerrors.ErrInvalidRequestedKey)
			}
		}
	}

	if params.BodyLen > s.validationConfig.PrivilegedMaxBodySize() {
		return domainerrors.ErrBodyTooLarge
	}

	if params.TTL > s.validationConfig.PrivilegedMaxTTL() {
		return domainerrors.ErrInvalidTTL
	}

	if params.RequestedKeyLength < s.validationConfig.PrivilegedMinKeyLength() {
		return domainerrors.ErrInvalidRequestedKeyLength
	}

	return s.validateCommonRequestParams(params)
}

func (s *CacheService) validateUnprivilegedRequestParams(params objectvalue.CacheRequestParams) error {
	if params.RequestedKey != "" {
		return domainerrors.ErrNonAuthorized
	}

	if params.BodyLen > s.validationConfig.UnprivilegedMaxBodySize() {
		return domainerrors.ErrBodyTooLarge
	}

	if params.TTL < s.validationConfig.MinTTL() {
		return domainerrors.ErrInvalidTTL
	}

	if params.TTL > s.validationConfig.UnprivilegedMaxTTL() {
		return domainerrors.ErrInvalidTTL
	}

	if params.RequestedKeyLength < s.validationConfig.UnprivilegedMinKeyLength() {
		return domainerrors.ErrInvalidRequestedKeyLength
	}

	return s.validateCommonRequestParams(params)
}

func (s *CacheService) validateCommonRequestParams(params objectvalue.CacheRequestParams) error {
	if params.RequestedKeyLength > s.validationConfig.MaxKeyLength() {
		return domainerrors.ErrInvalidRequestedKeyLength
	}

	return nil
}

func (s *CacheService) logAPIKeyUsage(apikeyID string, params objectvalue.CacheRequestParams) {
	var reason apikeys.UsageReason

	switch {
	case params.TTL == 0:
		reason = apikeys.UsageReason_PERSISTKEY
	case params.RequestedKeyLength < s.validationConfig.UnprivilegedMinKeyLength():
		reason = apikeys.UsageReason_CUSTOMKEYLEN
	case params.RequestedKey != "":
		reason = apikeys.UsageReason_CUSTOMKEY
	case params.BodyLen > s.validationConfig.UnprivilegedMaxBodySize():
		reason = apikeys.UsageReason_LARGEBODY
	default:
		return
	}

	event := event.NewAPIKeyUsedEvent(apikeyID, reason, params.SourceIP)
	s.eventPublisher.NotifyAll(event)

	s.logger.Info("Sent apikey usage message", "apikey", apikeyID)
}
