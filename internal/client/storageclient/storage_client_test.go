package storageclient

import (
	"crypto/md5" //nolint:gosec
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCalculateContentHash(t *testing.T) {
	data := []byte("testdata")
	hash := md5.Sum(data) //nolint:gosec
	expected := base64.StdEncoding.EncodeToString(hash[:])
	actual := calculateContentHash(data)
	assert.Equal(t, expected, actual)
}
