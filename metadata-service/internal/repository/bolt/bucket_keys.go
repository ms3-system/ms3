package bolt

func ownerIndexKey(ownerID, bucketName string) ([]byte, error) {
	if err := validateKeyComponent("owner_id", ownerID); err != nil {
		return nil, err
	}
	if err := validateKeyComponent("bucket_name", bucketName); err != nil {
		return nil, err
	}

	key := make([]byte, 0, len(ownerID)+1+len(bucketName))
	key = append(key, ownerID...)
	key = append(key, keySeparator)
	key = append(key, bucketName...)
	return key, nil
}

func ownerIndexPrefix(ownerID string) ([]byte, error) {
	if err := validateKeyComponent("owner_id", ownerID); err != nil {
		return nil, err
	}

	prefix := make([]byte, 0, len(ownerID)+1)
	prefix = append(prefix, ownerID...)
	prefix = append(prefix, keySeparator)
	return prefix, nil
}
