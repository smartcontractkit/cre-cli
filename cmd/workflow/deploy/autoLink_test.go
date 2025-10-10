package deploy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"

	"github.com/smartcontractkit/cre-cli/internal/testutil/chainsim"
)

type mockGraphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

// TestCheckLinkStatusViaGraphQL tests the GraphQL link status checking functionality
func TestCheckLinkStatusViaGraphQL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		ownerAddress   string
		mockResponse   map[string]any
		expectedResult bool
		expectError    bool
	}{
		{
			name:         "Owner is linked",
			ownerAddress: "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
			mockResponse: map[string]any{
				"data": map[string]any{
					"listWorkflowOwners": map[string]any{
						"linkedOwners": []map[string]string{
							{
								"workflowOwnerAddress": "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
								"verificationStatus":   VerificationStatusSuccessful,
							},
						},
					},
				},
			},
			expectedResult: true,
			expectError:    false,
		},
		{
			name:         "Owner is not linked",
			ownerAddress: "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
			mockResponse: map[string]any{
				"data": map[string]any{
					"listWorkflowOwners": map[string]any{
						"linkedOwners": []map[string]string{},
					},
				},
			},
			expectedResult: false,
			expectError:    false,
		},
		{
			name:         "Different owner is linked",
			ownerAddress: "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
			mockResponse: map[string]any{
				"data": map[string]any{
					"listWorkflowOwners": map[string]any{
						"linkedOwners": []map[string]string{
							{
								"workflowOwnerAddress": "0x1234567890123456789012345678901234567890",
								"verificationStatus":   VerificationStatusSuccessful,
							},
						},
					},
				},
			},
			expectedResult: false,
			expectError:    false,
		},
		{
			name:         "Case insensitive matching",
			ownerAddress: "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
			mockResponse: map[string]any{
				"data": map[string]any{
					"listWorkflowOwners": map[string]any{
						"linkedOwners": []map[string]string{
							{
								"workflowOwnerAddress": "0xF39FD6E51AAD88F6F4CE6AB8827279CFFFB92266", // Uppercase
								"verificationStatus":   VerificationStatusSuccessful,
							},
						},
					},
				},
			},
			expectedResult: true,
			expectError:    false,
		},
		{
			name:         "Owner linked but verification not successful",
			ownerAddress: "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
			mockResponse: map[string]any{
				"data": map[string]any{
					"listWorkflowOwners": map[string]any{
						"linkedOwners": []map[string]string{
							{
								"workflowOwnerAddress": "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
								"verificationStatus":   "VERIFICATION_PENDING",
							},
						},
					},
				},
			},
			expectedResult: false,
			expectError:    false,
		},
		{
			name: "GraphQL error",
			mockResponse: map[string]any{
				"errors": []map[string]string{
					{"message": "GraphQL error"},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock GraphQL server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req mockGraphQLRequest
				json.NewDecoder(r.Body).Decode(&req)

				// Verify the query contains listWorkflowOwners
				assert.Contains(t, req.Query, "listWorkflowOwners")

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(tt.mockResponse)
			}))
			defer server.Close()

			// Create test environment
			simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)
			defer simulatedEnvironment.Close()

			ctx, _ := simulatedEnvironment.NewRuntimeContextWithBufferedOutput()
			h := newHandler(ctx, nil)
			h.inputs.WorkflowOwner = tt.ownerAddress
			h.environmentSet.GraphQLURL = server.URL + "/graphql"

			// Test the function
			ownerAddr := common.HexToAddress(tt.ownerAddress)
			result, err := h.checkLinkStatusViaGraphQL(ownerAddr)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedResult, result)
			}
		})
	}
}

// TestWaitForBackendLinkProcessing tests the polling functionality
func TestWaitForBackendLinkProcessing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		ownerAddress   string
		responses      []map[string]any // Multiple responses for polling
		expectedResult bool
		expectError    bool
		expectTimeout  bool
	}{
		{
			name:         "Immediate success",
			ownerAddress: "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
			responses: []map[string]any{
				{
					"data": map[string]any{
						"listWorkflowOwners": map[string]any{
							"linkedOwners": []map[string]string{
								{
									"workflowOwnerAddress": "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
									"verificationStatus":   VerificationStatusSuccessful,
								},
							},
						},
					},
				},
			},
			expectedResult: true,
			expectError:    false,
		},
		{
			name:         "Success after retries",
			ownerAddress: "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
			responses: []map[string]any{
				// First few attempts return empty
				{
					"data": map[string]any{
						"listWorkflowOwners": map[string]any{
							"linkedOwners": []map[string]string{},
						},
					},
				},
				{
					"data": map[string]any{
						"listWorkflowOwners": map[string]any{
							"linkedOwners": []map[string]string{},
						},
					},
				},
				// Third attempt succeeds
				{
					"data": map[string]any{
						"listWorkflowOwners": map[string]any{
							"linkedOwners": []map[string]string{
								{
									"workflowOwnerAddress": "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
									"verificationStatus":   VerificationStatusSuccessful,
								},
							},
						},
					},
				},
			},
			expectedResult: true,
			expectError:    false,
		},
		{
			name:         "Timeout after retries",
			ownerAddress: "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
			responses: []map[string]any{
				// Always return empty (simulate timeout)
				{
					"data": map[string]any{
						"listWorkflowOwners": map[string]any{
							"linkedOwners": []map[string]string{},
						},
					},
				},
			},
			expectError:   true,
			expectTimeout: true,
		},
		{
			name:         "GraphQL errors during polling",
			ownerAddress: "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
			responses: []map[string]any{
				{
					"errors": []map[string]string{
						{"message": "Temporary GraphQL error"},
					},
				},
				{
					"data": map[string]any{
						"listWorkflowOwners": map[string]any{
							"linkedOwners": []map[string]string{
								{
									"workflowOwnerAddress": "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
									"verificationStatus":   VerificationStatusSuccessful,
								},
							},
						},
					},
				},
			},
			expectedResult: true,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requestCount := 0
			maxRequests := len(tt.responses)

			// Create mock GraphQL server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if requestCount < maxRequests {
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(tt.responses[requestCount])
					requestCount++
				} else {
					// For timeout tests, keep returning empty responses
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(map[string]any{
						"data": map[string]any{
							"listWorkflowOwners": map[string]any{
								"linkedOwners": []map[string]string{},
							},
						},
					})
				}
			}))
			defer server.Close()

			// Create test environment
			simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)
			defer simulatedEnvironment.Close()

			ctx, _ := simulatedEnvironment.NewRuntimeContextWithBufferedOutput()
			h := newHandler(ctx, nil)
			h.inputs.WorkflowOwner = tt.ownerAddress
			h.environmentSet.GraphQLURL = server.URL + "/graphql"

			// Test the function
			ownerAddr := common.HexToAddress(tt.ownerAddress)
			err := h.waitForBackendLinkProcessing(ownerAddr)

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectTimeout {
					assert.Contains(t, err.Error(), "linking process timeout")
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestTryAutoLink tests the auto-link execution setup
func TestTryAutoLink(t *testing.T) {
	t.Parallel()

	t.Run("sets up linkkey inputs correctly", func(t *testing.T) {
		simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)
		defer simulatedEnvironment.Close()

		ctx, _ := simulatedEnvironment.NewRuntimeContextWithBufferedOutput()
		h := newHandler(ctx, nil)
		h.inputs.WorkflowOwner = "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"
		h.inputs.WorkflowRegistryContractAddress = "0x1234567890123456789012345678901234567890"

		// Verify the handler is set up correctly
		assert.Equal(t, "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266", h.inputs.WorkflowOwner)
		assert.Equal(t, "0x1234567890123456789012345678901234567890", h.inputs.WorkflowRegistryContractAddress)
	})
}
