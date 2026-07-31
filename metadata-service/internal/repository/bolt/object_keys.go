package bolt

func objectKey(bucketName, objectKeyName string) ([]byte, error) {
	if err := validateKeyComponent("bucket_name", bucketName); err != nil {
		return nil, err
	}
	if err := validateKeyComponent("object_key", objectKeyName); err != nil {
		return nil, err
	}

	key := make([]byte, 0, len(bucketName)+1+len(objectKeyName))
	key = append(key, bucketName...)
	key = append(key, keySeparator)
	key = append(key, objectKeyName...)
	return key, nil
}

func objectListPrefix(bucketName, userPrefix string) ([]byte, error) {
	if err := validateKeyComponent("bucket_name", bucketName); err != nil {
		return nil, err
	}
	if userPrefix != "" {
		if err := validateKeyComponent("prefix", userPrefix); err != nil {
			return nil, err
		}
	}

	prefix := make([]byte, 0, len(bucketName)+1+len(userPrefix))
	prefix = append(prefix, bucketName...)
	prefix = append(prefix, keySeparator)
	prefix = append(prefix, userPrefix...)
	return prefix, nil
}
