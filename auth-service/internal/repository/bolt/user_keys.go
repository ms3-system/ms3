package bolt

func UserKey(userID string) ([]byte, error) {
	if err := ValidateKeyComponent("user_id", userID); err != nil {
		return nil, err
	}
	return []byte(userID), nil
}

func UsernameKey(username string) ([]byte, error) {
	if err := ValidateKeyComponent("username", username); err != nil {
		return nil, err
	}
	return []byte(username), nil
}
