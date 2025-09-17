package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUint8Array(t *testing.T) {
	type Uint8ArrayTestStruct struct {
		DecimalsArr string `validate:"uint8_string_array"`
	}

	v, err := NewValidator()
	assert.NoError(t, err, "Expected no error during validator initialization")

	tests := []struct {
		name             string
		input            Uint8ArrayTestStruct
		wantError        bool
		wantErrorKeys    []string
		wantErrorDetails []string
	}{
		{
			name:             "valid uint8 string array",
			input:            Uint8ArrayTestStruct{DecimalsArr: "[8, 0,0,1 ] "},
			wantError:        false,
			wantErrorKeys:    []string{},
			wantErrorDetails: []string{},
		},
		{
			name:             "invalid uint8 string array - overflow",
			input:            Uint8ArrayTestStruct{DecimalsArr: "[888, 0,0,1 ] "},
			wantError:        true,
			wantErrorKeys:    []string{"Uint8ArrayTestStruct.DecimalsArr"},
			wantErrorDetails: []string{"DecimalsArr must be a valid uint8 array wrapped within brackets: [888, 0,0,1 ] "},
		},
		{
			name:             "invalid uint8 string array - underflow",
			input:            Uint8ArrayTestStruct{DecimalsArr: "[-1, 0,0,1 ] "},
			wantError:        true,
			wantErrorKeys:    []string{"Uint8ArrayTestStruct.DecimalsArr"},
			wantErrorDetails: []string{"DecimalsArr must be a valid uint8 array wrapped within brackets: [-1, 0,0,1 ] "},
		},
		{
			name:             "invalid uint8 string array - empty array",
			input:            Uint8ArrayTestStruct{DecimalsArr: "[ ] "},
			wantError:        true,
			wantErrorKeys:    []string{"Uint8ArrayTestStruct.DecimalsArr"},
			wantErrorDetails: []string{"DecimalsArr must be a valid uint8 array wrapped within brackets: [ ] "},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err = v.Struct(tt.input)

			if !tt.wantError && err != nil {
				assert.NoError(t, err, "test should not result in errors")
			}

			if tt.wantError {
				assert.Error(t, err, "test should result in error")

				for i := range tt.wantErrorKeys {
					AssertErrors(t, err, tt.wantErrorKeys[i], tt.wantErrorDetails[i], v)
				}
			}
		})
	}
}
