package store

const (
	BoltBucketRecords  = "buckets"
	BoltBucketOwnerIdx = "bucket_owner_index"
	BoltBucketObjects  = "objects"
	BoltBucketMeta     = "meta"
)

var allBoltBuckets = []string{
	BoltBucketRecords,
	BoltBucketOwnerIdx,
	BoltBucketObjects,
	BoltBucketMeta,
}
