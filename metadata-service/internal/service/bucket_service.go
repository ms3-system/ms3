package service

import (
	"context"
	"fmt"
	"log/slog"
	"metadata-service/internal/model"
	"metadata-service/internal/repository"
	"regexp"

	"github.com/google/uuid"
)

var bucketNameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$`)

type bucketService struct {
	buckets repository.BucketRepository
	logger  *slog.Logger
}

func NewBucketService(buckets repository.BucketRepository, logger *slog.Logger) BucketService {
	return &bucketService{
		buckets: buckets,
		logger:  logger.With(slog.String("component", "service.bucket")),
	}
}

func (s *bucketService) CreateBucket(ctx context.Context, name, ownerID string) (model.Bucket, error) {
	log := s.logger.With(slog.String("bucket_name", name), slog.String("owner_id", ownerID))

	if !bucketNameRE.MatchString(name) {
		log.Debug("create rejected: invalid name")
		return model.Bucket{}, fmt.Errorf("%w: bucket name must be 3-63 lowercase alphanumeric/hyphen characters, got %q", ErrInvalidInput, name)
	}
	if ownerID == "" {
		log.Debug("create rejected: missing owner_id")
		return model.Bucket{}, fmt.Errorf("%w: owner_id is required", ErrInvalidInput)
	}

	b := model.Bucket{
		ID:      uuid.NewString(),
		Name:    name,
		OwnerID: ownerID,
	}

	if err := s.buckets.Create(ctx, &b); err != nil {
		return model.Bucket{}, err
	}

	log.Info("bucket created")
	return b, nil
}

func (s *bucketService) GetBucket(ctx context.Context, name string) (model.Bucket, error) {
	if name == "" {
		return model.Bucket{}, fmt.Errorf("%w: bucket name is required", ErrInvalidInput)
	}
	return s.buckets.GetByName(ctx, name)
}

func (s *bucketService) ListBuckets(ctx context.Context, ownerID string) ([]model.Bucket, error) {
	if ownerID == "" {
		return nil, fmt.Errorf("%w: owner_id is required", ErrInvalidInput)
	}
	return s.buckets.ListByOwner(ctx, ownerID)
}

func (s *bucketService) DeleteBucket(ctx context.Context, name string) error {
	if name == "" {
		return fmt.Errorf("%w: bucket name is required", ErrInvalidInput)
	}

	err := s.buckets.Delete(ctx, name)
	if err == nil {
		s.logger.Info("bucket deleted", slog.String("bucket_name", name))
	}
	return err
}
