package bolt

import (
	"bytes"
	"fmt"
)

const keySeparator = 0x00

func ValidateKeyComponent(fieldName, value string) error {
	if bytes.IndexByte([]byte(value), keySeparator) != -1 {
		return fmt.Errorf("%s must not contain a null byte: %q", fieldName, value)
	}
	return nil
}
