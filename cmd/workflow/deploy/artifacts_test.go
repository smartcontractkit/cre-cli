package deploy

import (
	//nolint:gosec
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/testutil/chainsim"
)

func mockGraphQL() {
	httpmock.RegisterResponder("POST", "http://graphql.endpoint",
		func(req *http.Request) (*http.Response, error) {
			bodyBytes, _ := io.ReadAll(req.Body)
			var gqlReq GraphQLRequest
			_ = json.Unmarshal(bodyBytes, &gqlReq)

			if strings.Contains(gqlReq.Query, "mutation GeneratePresignedPostUrlForArtifact") {
				resp, _ := httpmock.NewJsonResponse(200, map[string]interface{}{
					"data": map[string]interface{}{
						"generatePresignedPostUrlForArtifact": map[string]interface{}{
							"presignedPostUrl":    "http://origin/upload",
							"presignedPostFields": []map[string]string{{"key": "key1", "value": "val1"}},
						},
					},
				})
				return resp, nil
			}

			if strings.Contains(gqlReq.Query, "mutation GenerateUnsignedGetUrlForArtifact") {
				resp, _ := httpmock.NewJsonResponse(200, map[string]interface{}{
					"data": map[string]interface{}{
						"generateUnsignedGetUrlForArtifact": map[string]interface{}{
							"unsignedGetUrl": "http://origin/get",
						},
					},
				})
				return resp, nil
			}

			resp, _ := httpmock.NewJsonResponse(400, map[string]interface{}{
				"errors": []map[string]string{{"message": "Unsupported GraphQL query"}},
			})
			return resp, nil
		},
	)
}

type GraphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
}

func TestUpload_SuccessAndErrorCases(t *testing.T) {
	httpmock.Activate()
	t.Cleanup(httpmock.DeactivateAndReset)

	simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)
	ctx, buf := simulatedEnvironment.NewRuntimeContextWithBufferedOutput()
	h := newHandler(ctx, buf)
	h.inputs.WorkflowOwner = chainsim.TestAddress
	h.inputs.WorkflowName = "test_workflow"
	h.inputs.DonFamily = "test_label"

	// Manually construct settings to fix nil pointer issue
	h.settings = &settings.Settings{
		Workflow: settings.WorkflowSettings{
			UserWorkflowSettings: struct {
				WorkflowOwnerAddress string `mapstructure:"workflow-owner-address" yaml:"workflow-owner-address"`
				WorkflowOwnerType    string `mapstructure:"workflow-owner-type" yaml:"workflow-owner-type"`
				WorkflowName         string `mapstructure:"workflow-name" yaml:"workflow-name"`
			}{
				WorkflowOwnerAddress: chainsim.TestAddress,
				WorkflowOwnerType:    "eoa",
				WorkflowName:         "test_workflow",
			},
		},
		StorageSettings: settings.WorkflowStorageSettings{
			CREStorage: settings.CREStorageSettings{
				ServiceTimeout: 0,
				HTTPTimeout:    0,
			},
		},
	}

	// Mock Auth0 token refresh endpoint with properly formatted JWT
	mockJWT := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyLCJleHAiOjk5OTk5OTk5OTl9.Lqfm3qWjP6QadY_KjdRfXE4ue9GaGrA6JlNYKm6YhSw"
	authResp, _ := httpmock.NewJsonResponse(200, map[string]interface{}{
		"access_token": mockJWT,
		"token_type":   "Bearer",
		"expires_in":   3600,
	})
	httpmock.RegisterResponder("POST", "https://cre-staging.us.auth0.com/oauth/token",
		httpmock.ResponderFromResponse(authResp))

	mockGraphQL()

	// Mock origin upload response
	httpmock.RegisterResponder("POST", "http://origin/upload",
		httpmock.NewStringResponder(201, ""))

	// Patch settings to use mock GraphQL endpoint
	h.environmentSet.GraphQLURL = "http://graphql.endpoint"

	// Set up credentials with the mock JWT token
	h.credentials.Tokens = &credentials.CreLoginTokenSet{
		AccessToken:  mockJWT,
		RefreshToken: "mock-refresh-token",
		TokenType:    "Bearer",
		ExpiresIn:    3600,
	}

	// Success case : uploading binary and config data
	h.workflowArtifact = &workflowArtifact{
		BinaryData: []byte("binarydata"),
		ConfigData: []byte("configdata"),
		WorkflowID: "workflow-id",
	}
	err := h.UploadArtifacts()
	require.NoError(t, err)
	require.Equal(t, "http://origin/get", h.inputs.BinaryURL)
	require.Equal(t, "http://origin/get", *h.inputs.ConfigURL)

	// Success: empty ConfigData
	h.workflowArtifact = &workflowArtifact{
		BinaryData: []byte("binarydata"),
		ConfigData: nil,
		WorkflowID: "workflow-id",
	}
	err = h.UploadArtifacts()
	require.NoError(t, err)

	// Error: workflowArtifact is nil
	h.workflowArtifact = nil
	err = h.UploadArtifacts()
	require.ErrorContains(t, err, "workflowArtifact is nil")

	// Error: empty BinaryData
	h.workflowArtifact = &workflowArtifact{
		BinaryData: nil,
		ConfigData: []byte("configdata"),
		WorkflowID: "workflow-id",
	}
	err = h.UploadArtifacts()
	require.ErrorContains(t, err, "uploading binary artifact: content is empty for artifactType BINARY")

	// Error: workflowID is empty
	h.workflowArtifact = &workflowArtifact{
		BinaryData: []byte("binarydata"),
		ConfigData: []byte("configdata"),
		WorkflowID: "",
	}
	err = h.UploadArtifacts()
	require.ErrorContains(t, err, "workflowID is empty")

}

func TestUploadArtifactToStorageService_OriginError(t *testing.T) {
	httpmock.Activate()
	t.Cleanup(httpmock.DeactivateAndReset)

	simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)
	runtimeContext, buf := simulatedEnvironment.NewRuntimeContextWithBufferedOutput()
	h := newHandler(runtimeContext, buf)
	h.inputs.WorkflowOwner = chainsim.TestAddress
	h.inputs.WorkflowName = "test_workflow"
	h.inputs.DonFamily = "test_label"

	// Manually construct settings to fix nil pointer issue
	h.settings = &settings.Settings{
		Workflow: settings.WorkflowSettings{
			UserWorkflowSettings: struct {
				WorkflowOwnerAddress string `mapstructure:"workflow-owner-address" yaml:"workflow-owner-address"`
				WorkflowOwnerType    string `mapstructure:"workflow-owner-type" yaml:"workflow-owner-type"`
				WorkflowName         string `mapstructure:"workflow-name" yaml:"workflow-name"`
			}{
				WorkflowOwnerAddress: chainsim.TestAddress,
				WorkflowOwnerType:    "eoa",
				WorkflowName:         "test_workflow",
			},
		},
		StorageSettings: settings.WorkflowStorageSettings{
			CREStorage: settings.CREStorageSettings{
				ServiceTimeout: 0,
				HTTPTimeout:    0,
			},
		},
	}

	// Mock Auth0 token refresh endpoint with properly formatted JWT
	mockJWT := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyLCJleHAiOjk5OTk5OTk5OTl9.Lqfm3qWjP6QadY_KjdRfXE4ue9GaGrA6JlNYKm6YhSw"
	authResp, _ := httpmock.NewJsonResponse(200, map[string]interface{}{
		"access_token": mockJWT,
		"token_type":   "Bearer",
		"expires_in":   3600,
	})
	httpmock.RegisterResponder("POST", "https://cre-staging.us.auth0.com/oauth/token",
		httpmock.ResponderFromResponse(authResp))

	mockGraphQL()

	// Mock origin upload response
	httpmock.RegisterResponder("POST", "http://origin/upload",
		httpmock.NewStringResponder(501, ""))

	// Patch settings to use mock GraphQL endpoint
	h.environmentSet.GraphQLURL = "http://graphql.endpoint"

	// Set up credentials with the mock JWT token
	h.credentials.Tokens = &credentials.CreLoginTokenSet{
		AccessToken:  mockJWT,
		RefreshToken: "mock-refresh-token",
		TokenType:    "Bearer",
		ExpiresIn:    3600,
	}

	h.workflowArtifact = &workflowArtifact{
		BinaryData: []byte("binarydata"),
		ConfigData: []byte("configdata"),
		WorkflowID: "workflow-id",
	}
	err := h.UploadArtifacts()
	require.ErrorContains(t, err, "upload to origin")
}

func TestUploadArtifactToStorageService_AlreadyExistsError(t *testing.T) {
	httpmock.Activate()
	t.Cleanup(httpmock.DeactivateAndReset)

	simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)
	runtimeContext, buf := simulatedEnvironment.NewRuntimeContextWithBufferedOutput()
	h := newHandler(runtimeContext, buf)
	h.inputs.WorkflowOwner = chainsim.TestAddress
	h.inputs.WorkflowName = "test_workflow"
	h.inputs.DonFamily = "test_label"

	// Manually construct settings to fix nil pointer issue
	h.settings = &settings.Settings{
		Workflow: settings.WorkflowSettings{
			UserWorkflowSettings: struct {
				WorkflowOwnerAddress string `mapstructure:"workflow-owner-address" yaml:"workflow-owner-address"`
				WorkflowOwnerType    string `mapstructure:"workflow-owner-type" yaml:"workflow-owner-type"`
				WorkflowName         string `mapstructure:"workflow-name" yaml:"workflow-name"`
			}{
				WorkflowOwnerAddress: chainsim.TestAddress,
				WorkflowOwnerType:    "eoa",
				WorkflowName:         "test_workflow",
			},
		},
		StorageSettings: settings.WorkflowStorageSettings{
			CREStorage: settings.CREStorageSettings{
				ServiceTimeout: 0,
				HTTPTimeout:    0,
			},
		},
	}

	// Mock Auth0 token refresh endpoint with properly formatted JWT
	mockJWT := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyLCJleHAiOjk5OTk5OTk5OTl9.Lqfm3qWjP6QadY_KjdRfXE4ue9GaGrA6JlNYKm6YhSw"
	authResp, _ := httpmock.NewJsonResponse(200, map[string]interface{}{
		"access_token": mockJWT,
		"token_type":   "Bearer",
		"expires_in":   3600,
	})
	httpmock.RegisterResponder("POST", "https://cre-staging.us.auth0.com/oauth/token",
		httpmock.ResponderFromResponse(authResp))

	httpmock.RegisterResponder("POST", "http://graphql.endpoint",
		func(req *http.Request) (*http.Response, error) {
			bodyBytes, _ := io.ReadAll(req.Body)
			var gqlReq GraphQLRequest
			_ = json.Unmarshal(bodyBytes, &gqlReq)

			if strings.Contains(gqlReq.Query, "mutation GeneratePresignedPostUrlForArtifact") {
				return nil, errors.New("generate presigned post url: graphql: rpc error: code = Internal desc = failed to generate presigned URL: key artifacts/workflow-id/binary.wasm already exists")
			}

			if strings.Contains(gqlReq.Query, "mutation GenerateUnsignedGetUrlForArtifact") {
				resp, _ := httpmock.NewJsonResponse(200, map[string]interface{}{
					"data": map[string]interface{}{
						"generateUnsignedGetUrlForArtifact": map[string]interface{}{
							"unsignedGetUrl": "http://origin/get",
						},
					},
				})
				return resp, nil
			}

			resp, _ := httpmock.NewJsonResponse(400, map[string]interface{}{
				"errors": []map[string]string{{"message": "Unsupported GraphQL query"}},
			})
			return resp, nil
		},
	)

	// Mock origin upload response
	httpmock.RegisterResponder("POST", "http://origin/upload",
		httpmock.NewStringResponder(501, ""))

	// Patch settings to use mock GraphQL endpoint
	h.environmentSet.GraphQLURL = "http://graphql.endpoint"

	// Set up credentials with the mock JWT token
	h.credentials.Tokens = &credentials.CreLoginTokenSet{
		AccessToken:  mockJWT,
		RefreshToken: "mock-refresh-token",
		TokenType:    "Bearer",
		ExpiresIn:    3600,
	}

	h.workflowArtifact = &workflowArtifact{
		BinaryData: []byte("binarydata"),
		ConfigData: []byte("configdata"),
		WorkflowID: "workflow-id",
	}
	err := h.UploadArtifacts()
	require.NoError(t, err)
	require.Equal(t, "http://origin/get", h.inputs.BinaryURL)
	require.Equal(t, "http://origin/get", *h.inputs.ConfigURL)
}
