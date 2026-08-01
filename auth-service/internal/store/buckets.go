package store

const (
	BoltBucketUsers       = "users"
	BoltBucketUsernameIdx = "usernames"
	BoltBucketCredentials = "credentials"
	BoltBucketMeta        = "meta"
)

var allBoltBuckets = []string{
	BoltBucketUsers,
	BoltBucketUsernameIdx,
	BoltBucketCredentials,
	BoltBucketMeta,
}
