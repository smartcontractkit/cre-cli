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

	"github.com/smartcontractkit/dev-platform/internal/testutil/chainsim"
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

	mockGraphQL()

	// Mock origin upload response
	httpmock.RegisterResponder("POST", "http://origin/upload",
		httpmock.NewStringResponder(201, ""))

	// Patch settings to use mock GraphQL endpoint
	h.environmentSet.GraphQLURL = "http://graphql.endpoint"

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

	mockGraphQL()

	// Mock origin upload response
	httpmock.RegisterResponder("POST", "http://origin/upload",
		httpmock.NewStringResponder(501, ""))

	// Patch settings to use mock GraphQL endpoint
	h.environmentSet.GraphQLURL = "http://graphql.endpoint"

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
