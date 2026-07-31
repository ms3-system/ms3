package bolt

import "testing"

func TestValidateKeyComponent(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "plain string", value: "my-bucket", wantErr: false},
		{name: "empty string", value: "", wantErr: false},
		{name: "contains separator", value: "evil\x00malicious", wantErr: true},
		{name: "separator at start", value: "\x00leading", wantErr: true},
		{name: "separator at end", value: "trailing\x00", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateKeyComponent("field", tt.value)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateKeyComponent(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}
