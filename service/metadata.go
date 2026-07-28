package service

import (
	"slices"

	"github.com/naseyro/minis3/api"
)

type MetadataInterface interface {
	CreateBucket(string)
	GetBucket(string) api.Bucket
	GetBuckets() []api.Bucket
	DeleteBucket(string)
	CreateObject(string)
	GetObject(string) api.ObjectMetadata
	GetObjects() []api.ObjectMetadata
	DeleteObject(string)
}

type metadataService struct {
	ObjectsMetadata []api.ObjectMetadata
	BucketsMetadata []api.Bucket
}

func (m *metadataService) CreateBucket(name string) {
	if _, bucket := m.GetBucket(name); bucket != nil {
		return
	}
	bucket := api.NewBucket(name)
	m.BucketsMetadata = append(m.BucketsMetadata, *bucket)
}

func (m *metadataService) GetBuckets() []api.Bucket {
	return m.BucketsMetadata
}

func (m *metadataService) GetBucket(name string) (int, *api.Bucket) {
	for i, b := range m.BucketsMetadata {
		if b.Name == name {
			return i, &b
		}
	}
	return 0, nil
}

func (m *metadataService) DeleteBucket(name string) {
	i, bucket := m.GetBucket(name)
	if bucket == nil {
		return
	}
	m.BucketsMetadata = slices.Delete(m.BucketsMetadata, i, i+1)
}

func (m *metadataService) CreateObject(name string) {

}
