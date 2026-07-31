package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"metadata-service/internal/model"
	"metadata-service/internal/repository"
)

type objectService struct {
	objects repository.ObjectRepository
	buckets repository.BucketRepository
	logger  *slog.Logger
}

func NewObjectService(objects repository.ObjectRepository, buckets repository.BucketRepository, logger *slog.Logger) ObjectService {
	return &objectService{
		objects: objects,
		buckets: buckets,
		logger:  logger.With(slog.String("component", "service.object")),
	}
}

func (s *objectService) PutObject(ctx context.Context, bucketName, key string, sizeBytes int64, etag, contentType, storageRef string) (model.Object, error) {
	log := s.logger.With(slog.String("bucket_name", bucketName), slog.String("object_key", key))

	if bucketName == "" {
		log.Debug("put rejected: missing bucket name")
		return model.Object{}, fmt.Errorf("%w: bucket name is required", ErrInvalidInput)
	}
	if key == "" {
		log.Debug("put rejected: missing object key")
		return model.Object{}, fmt.Errorf("%w: object key is required", ErrInvalidInput)
	}
	if sizeBytes < 0 {
		log.Debug("put rejected: negative size_bytes")
		return model.Object{}, fmt.Errorf("%w: size_bytes must be non-negative, got %d", ErrInvalidInput, sizeBytes)
	}
	if storageRef == "" {
		log.Debug("put rejected: missing storage_ref")
		return model.Object{}, fmt.Errorf("%w: storage_ref is required", ErrInvalidInput)
	}

	if _, err := s.buckets.GetByName(ctx, bucketName); err != nil {
		return model.Object{}, err
	}

	o := model.Object{
		ID:          uuid.NewString(),
		BucketName:  bucketName,
		ObjectKey:   key,
		SizeBytes:   sizeBytes,
		ETag:        etag,
		ContentType: contentType,
		StorageRef:  storageRef,
		VersionID:   "null",
		IsLatest:    true,
	}

	if err := s.objects.Put(ctx, &o); err != nil {
		return model.Object{}, err
	}

	log.Info("object put", slog.Int64("size_bytes", sizeBytes))
	return o, nil
}

func (s *objectService) GetObject(ctx context.Context, bucketName, key string) (model.Object, error) {
	if bucketName == "" {
		return model.Object{}, fmt.Errorf("%w: bucket name is required", ErrInvalidInput)
	}
	if key == "" {
		return model.Object{}, fmt.Errorf("%w: object key is required", ErrInvalidInput)
	}
	return s.objects.Get(ctx, bucketName, key)
}

func (s *objectService) ListObjects(ctx context.Context, bucketName, prefix string, limit int) ([]model.Object, error) {
	if bucketName == "" {
		return nil, fmt.Errorf("%w: bucket name is required", ErrInvalidInput)
	}
	return s.objects.List(ctx, bucketName, prefix, limit)
}

func (s *objectService) DeleteObject(ctx context.Context, bucketName, key string) error {
	if bucketName == "" {
		return fmt.Errorf("%w: bucket name is required", ErrInvalidInput)
	}
	if key == "" {
		return fmt.Errorf("%w: object key is required", ErrInvalidInput)
	}

	err := s.objects.Delete(ctx, bucketName, key)
	if err == nil {
		s.logger.Info("object deleted", slog.String("bucket_name", bucketName), slog.String("object_key", key))
	}
	return err
}
