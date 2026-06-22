package gateway

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateGatewayURL(t *testing.T) {
	require.NoError(t, ValidateGatewayURL("https://gateway.example.com/"))
	require.NoError(t, ValidateGatewayURL("https://gateway.example.com/v1"))
	require.NoError(t, ValidateGatewayURL("http://127.0.0.1:57244"))
	require.NoError(t, ValidateGatewayURL("http://localhost:8080"))
	require.NoError(t, ValidateGatewayURL("http://[::1]:8080"))

	err := ValidateGatewayURL("http://gateway.example.com/")
	require.Error(t, err)
	require.Contains(t, err.Error(), "https://")

	err = ValidateGatewayURL("http://10.0.0.1/")
	require.Error(t, err)
	require.Contains(t, err.Error(), "https://")

	err = ValidateGatewayURL("http://127.0.0.1.evil.com/")
	require.Error(t, err)
	require.Contains(t, err.Error(), "https://")

	err = ValidateGatewayURL("https://")
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing a host")

	err = ValidateGatewayURL("not-a-url")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid vault gateway URL")
}
