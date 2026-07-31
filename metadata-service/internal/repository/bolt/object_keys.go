package bolt

func ObjectKey(bucketName, objectKeyName string) ([]byte, error) {
	if err := ValidateKeyComponent("bucket_name", bucketName); err != nil {
		return nil, err
	}
	if err := ValidateKeyComponent("object_key", objectKeyName); err != nil {
		return nil, err
	}

	key := make([]byte, 0, len(bucketName)+1+len(objectKeyName))
	key = append(key, bucketName...)
	key = append(key, keySeparator)
	key = append(key, objectKeyName...)
	return key, nil
}

func ObjectListPrefix(bucketName, userPrefix string) ([]byte, error) {
	if err := ValidateKeyComponent("bucket_name", bucketName); err != nil {
		return nil, err
	}
	if userPrefix != "" {
		if err := ValidateKeyComponent("prefix", userPrefix); err != nil {
			return nil, err
		}
	}

	prefix := make([]byte, 0, len(bucketName)+1+len(userPrefix))
	prefix = append(prefix, bucketName...)
	prefix = append(prefix, keySeparator)
	prefix = append(prefix, userPrefix...)
	return prefix, nil
}
