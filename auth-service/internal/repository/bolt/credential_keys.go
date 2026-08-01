package bolt

func CredentialKey(accessKey string) ([]byte, error) {
	if err := ValidateKeyComponent("access_key", accessKey); err != nil {
		return nil, err
	}
	return []byte(accessKey), nil
}
