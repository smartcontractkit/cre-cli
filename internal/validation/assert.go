package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// AssertErrors is a helper function to assert that the validation errors are as expected.
func AssertErrors(t *testing.T, err error, key, detail string, v *Validator) {
	t.Helper()

	errs := v.ParseValidationErrors(err)
	AssertValidationErrs(t, errs, key, detail)
}

func AssertValidationErrs(t *testing.T, errs ValidationErrors, key string, detail string) {
	t.Helper()

	found := false
	var ve ValidationError

	for i := 0; i < len(errs); i++ {
		if errs[i].Field == key {
			found = true
			ve = errs[i]
			break
		}
	}

	assert.True(t, found)
	assert.Equal(t, key, ve.Field)
	assert.Equal(t, detail, ve.Detail)
}
