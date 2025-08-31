//go:build integration

package service

// import (
// 	"context"
// 	"testing"
//
// 	"github.com/google/uuid"
// 	"github.com/stretchr/testify/assert"
// 	"github.com/stretchr/testify/mock"
// 	"github.com/stretchr/testify/require"
//
// 	"github.com/thek4n/paste.thek4n.ru/internal/infrastructure/repository"
// 	"github.com/thek4n/paste.thek4n.ru/internal/domain/config"
// 	"github.com/thek4n/paste.thek4n.ru/internal/domain/domainerrors"
// 	"github.com/thek4n/paste.thek4n.ru/internal/domain/event"
// 	"github.com/thek4n/paste.thek4n.ru/internal/domain/logger"
// 	"github.com/thek4n/paste.thek4n.ru/internal/domain/objectvalue"
// 	"github.com/thek4n/paste.thek4n.ru/pkg/apikeys"
// )
//
// func TestCacheService_Serve(t *testing.T) {
// 	t.Parallel()
//
// 	ctx := context.Background()
// 	publisher := event.NewPublisher()
//
// 	// Use in-memory repositories
// 	recordRepo := repository.NewRedisRecordRepository()
// 	quotaRepo := repository.NewRedisQuotaRepository()
// 	apikeyRepo := repository.NewRedisAPIKeyRORepository()
//
// 	// Setup service
// 	svc := NewCacheService(
// 		recordRepo,
// 		quotaRepo,
// 		apikeyRepo,
// 		publisher,
// 		config.DefaultCacheValidationConfig{},
// 		config.DefaultQuotaConfig{},
// 		lgr,
// 	)
// }
