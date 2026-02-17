package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateECDSAPrivateKey(t *testing.T) {
	validator, err := NewValidator()
	assert.NoError(t, err, "Expected no error during validator initialization")

	type ECDSATestStruct struct {
		PrivateKey string `validate:"ecdsa_private_key"` // #nosec G117 -- test struct for validation testing
	}

	tests := []struct {
		name             string
		input            ECDSATestStruct
		wantError        bool
		wantErrorKeys    []string
		wantErrorDetails []string
	}{
		{
			name: "Valid ECDSA private key",
			input: ECDSATestStruct{
				PrivateKey: "4c0883a69102937d6234146e38a6aefbf95944c6e4d4013a6d287e4d739e7f9b",
			},
			wantError:        false,
			wantErrorKeys:    []string{},
			wantErrorDetails: []string{},
		},
		{
			name: "Valid ECDSA private key - case insensitive",
			input: ECDSATestStruct{
				PrivateKey: "4C0883A69102937D6234146e38a6aefbf95944c6e4d4013a6d287e4d739e7F9B",
			},
			wantError:        false,
			wantErrorKeys:    []string{},
			wantErrorDetails: []string{},
		},
		{
			name: "Invalid private key - starts with 0x",
			input: ECDSATestStruct{
				PrivateKey: "0x4c0883a69102937d6234146e38a6aefbf95944c6e4d4013a6d287e4d739e7f9b",
			},
			wantError:        true,
			wantErrorKeys:    []string{"ECDSATestStruct.PrivateKey"},
			wantErrorDetails: []string{"PrivateKey must be a valid ECDSA private key"},
		},
		{
			name: "Invalid private key - non-hex characters",
			input: ECDSATestStruct{
				PrivateKey: "invalid-private-key",
			},
			wantError:        true,
			wantErrorKeys:    []string{"ECDSATestStruct.PrivateKey"},
			wantErrorDetails: []string{"PrivateKey must be a valid ECDSA private key"},
		},
		{
			name: "Invalid private key - too short",
			input: ECDSATestStruct{
				PrivateKey: "4c0883a69102937d6234146e38a6aefbf",
			},
			wantError:        true,
			wantErrorKeys:    []string{"ECDSATestStruct.PrivateKey"},
			wantErrorDetails: []string{"PrivateKey must be a valid ECDSA private key"},
		},
		{
			name: "Invalid private key - too long",
			input: ECDSATestStruct{
				PrivateKey: "4c0883a69102937d6234146e38a6aefbf95944c6e4d4013a6d287e4d739e7f9b4c0883a69102937d6234146e38a6aefbf",
			},
			wantError:        true,
			wantErrorKeys:    []string{"ECDSATestStruct.PrivateKey"},
			wantErrorDetails: []string{"PrivateKey must be a valid ECDSA private key"},
		},
		{
			name: "Empty private key",
			input: ECDSATestStruct{
				PrivateKey: "",
			},
			wantError:        true,
			wantErrorKeys:    []string{"ECDSATestStruct.PrivateKey"},
			wantErrorDetails: []string{"PrivateKey must be a valid ECDSA private key"},
		},
		{
			name: "Invalid private key - valid hex but invalid for ECDSA",
			input: ECDSATestStruct{
				PrivateKey: "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
			},
			wantError:        true,
			wantErrorKeys:    []string{"ECDSATestStruct.PrivateKey"},
			wantErrorDetails: []string{"PrivateKey must be a valid ECDSA private key"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err = validator.Struct(tt.input)

			if !tt.wantError && err != nil {
				assert.NoError(t, err, "test should not result in errors")
			}

			if tt.wantError {
				assert.Error(t, err, "test should result in error")

				for i := range tt.wantErrorKeys {
					AssertErrors(t, err, tt.wantErrorKeys[i], tt.wantErrorDetails[i], validator)
				}
			}
		})
	}

	t.Run("Fails validation on a non-string argument", func(t *testing.T) {
		type TestStruct struct {
			PrivateKey int `validate:"ecdsa_private_key"`
		}

		input := TestStruct{PrivateKey: 42}

		defer func() {
			if r := recover(); r != nil {
				assert.Contains(t, r, "input field name is not a string", "Expected panic for non-string argument")
			} else {
				t.Error("Expected panic, but did not get one")
			}
		}()

		_ = validator.Struct(input)
	})
}
