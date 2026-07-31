package bolt

func OwnerIndexKey(ownerID, bucketName string) ([]byte, error) {
	if err := ValidateKeyComponent("owner_id", ownerID); err != nil {
		return nil, err
	}
	if err := ValidateKeyComponent("bucket_name", bucketName); err != nil {
		return nil, err
	}

	key := make([]byte, 0, len(ownerID)+1+len(bucketName))
	key = append(key, ownerID...)
	key = append(key, keySeparator)
	key = append(key, bucketName...)
	return key, nil
}

func OwnerIndexPrefix(ownerID string) ([]byte, error) {
	if err := ValidateKeyComponent("owner_id", ownerID); err != nil {
		return nil, err
	}

	prefix := make([]byte, 0, len(ownerID)+1)
	prefix = append(prefix, ownerID...)
	prefix = append(prefix, keySeparator)
	return prefix, nil
}
