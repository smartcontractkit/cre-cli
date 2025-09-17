package common

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"testing"

	"github.com/rs/zerolog"
	"github.com/test-go/testify/mock"

	"github.com/smartcontractkit/dev-platform/cmd/client"
)

func newMockHandler(t *testing.T) (*Handler, *MockClientFactory, *ecdsa.PrivateKey) {
	logger := zerolog.New(bytes.NewBufferString(""))
	mockClientFactory := new(MockClientFactory)
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate private key: %v", err)
	}
	h := &Handler{
		Log:           &logger,
		ClientFactory: mockClientFactory,
		PrivateKey:    privateKey,
		OwnerAddress:  "0xabc",
	}
	return h, mockClientFactory, privateKey
}

// --------- Factory mock ---------

type MockClientFactory struct {
	mock.Mock
}

func (m *MockClientFactory) NewCapabilitiesRegistryClient() (*client.CapabilitiesRegistryClient, error) {
	args := m.Called()
	return args.Get(0).(*client.CapabilitiesRegistryClient), args.Error(1)
}

func (m *MockClientFactory) NewWorkflowRegistryV2Client() (*client.WorkflowRegistryV2Client, error) {
	args := m.Called()
	var c *client.WorkflowRegistryV2Client
	if v := args.Get(0); v != nil {
		c = v.(*client.WorkflowRegistryV2Client)
	}
	return c, args.Error(1)
}

func (m *MockClientFactory) GetTxType() client.TxType {
	panic("not used in these tests")
}
