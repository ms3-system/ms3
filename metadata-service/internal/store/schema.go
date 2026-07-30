package store

import (
	"encoding/binary"
	"fmt"

	bolt "go.etcd.io/bbolt"
)

const currentSchemaVersion = 1

var schemaVersionKey = []byte("schema_version")

func checkSchemaVersion(tx *bolt.Tx) error {
	meta := tx.Bucket([]byte(BoltBucketMeta))

	raw := meta.Get(schemaVersionKey)
	if raw == nil {
		return meta.Put(schemaVersionKey, encodeVersion(currentSchemaVersion))
	}

	if len(raw) != 8 {
		return fmt.Errorf("invalid schema version encoding (len=%d)", len(raw))
	}

	version := decodeVersion(raw)
	switch {
	case version > currentSchemaVersion:
		return fmt.Errorf("db schema v%d is newer than this binary supports (v%d) — refusing to start",
			version, currentSchemaVersion)
	case version < currentSchemaVersion:
		// TODO: as breaking changes happen, add e.g. migrateV1toV2(tx)
		// calls here, in order, each updating both data and the stamped
		// version before returning.
		return fmt.Errorf("migration from schema v%d to v%d not implemented yet", version, currentSchemaVersion)
	default:
		return nil
	}
}

func encodeVersion(v uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, v)
	return b
}

func decodeVersion(b []byte) uint64 {
	return binary.BigEndian.Uint64(b)
}
