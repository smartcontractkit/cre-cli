package storageclient

import (
	"bytes"
	"context"
	"crypto/md5" //nolint:gosec
	"encoding/base64"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/machinebox/graphql"
	"github.com/rs/zerolog"

	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
)

type Client struct {
	graphql                 *graphqlclient.Client
	workflowRegistryAddress string
	workflowOwnerAddress    string
	chainSelector           uint64
	log                     *zerolog.Logger
	serviceTimeout          time.Duration
	httpTimeout             time.Duration
}

func New(graphql *graphqlclient.Client, workflowRegistryAddress string, workflowOwnerAddress string, chainSelector uint64, log *zerolog.Logger) *Client {
	return &Client{
		graphql:                 graphql,
		workflowRegistryAddress: workflowRegistryAddress,
		workflowOwnerAddress:    workflowOwnerAddress,
		chainSelector:           chainSelector,
		log:                     log,
		serviceTimeout:          time.Minute * 2,
		httpTimeout:             time.Minute * 1,
	}
}

type DeleteArtifactResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type GeneratePresignedPostUrlForArtifactResponse struct {
	PresignedPostURL    string `json:"presignedPostUrl"`
	PresignedPostFields []struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	} `json:"presignedPostFields"`
}

type GenerateUnsignedGetUrlForArtifactResponse struct {
	UnsignedGetUrl string `json:"unsignedGetUrl"`
}

type ArtifactType string

const (
	ArtifactTypeBinary ArtifactType = "BINARY"
	ArtifactTypeConfig ArtifactType = "CONFIG"
)

func (c *Client) SetServiceTimeout(timeout time.Duration) {
	c.serviceTimeout = timeout
}

func (c *Client) SetHTTPTimeout(timeout time.Duration) {
	c.httpTimeout = timeout
}

func (c *Client) CreateServiceContextWithTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), c.serviceTimeout)
}

func (c *Client) CreateHttpContextWithTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), c.httpTimeout)
}

func (c *Client) GeneratePostUrlForArtifact(workflowId string, artifactType ArtifactType, content []byte) (GeneratePresignedPostUrlForArtifactResponse, error) {
	const mutation = `
mutation GeneratePresignedPostUrlForArtifact($artifact: GeneratePresignedPostUrlRequest!) {
  generatePresignedPostUrlForArtifact(artifact: $artifact) {
    presignedPostUrl
    presignedPostFields {
      key
      value
    }
  }
}`
	contentHash := calculateContentHash(content)
	req := graphql.NewRequest(mutation)
	reqVariables := map[string]any{
		"workflowId":              workflowId,
		"artifactType":            artifactType,
		"contentHash":             contentHash,
		"workflowOwnerAddress":    c.workflowOwnerAddress,
		"workflowRegistryAddress": c.workflowRegistryAddress,
		"chainSelector":           fmt.Sprintf("%v", c.chainSelector),
	}
	req.Var("artifact", reqVariables)

	var container struct {
		GeneratePresignedPostUrlForArtifact GeneratePresignedPostUrlForArtifactResponse `json:"generatePresignedPostUrlForArtifact"`
	}

	ctx, cancel := c.CreateServiceContextWithTimeout()
	defer cancel()

	if err := c.graphql.
		Execute(ctx, req, &container); err != nil {
		return GeneratePresignedPostUrlForArtifactResponse{}, err
	}

	c.log.Debug().Interface("response", container).
		Msg("Received GraphQL response from generatePresignedPostUrlForArtifact")

	return container.GeneratePresignedPostUrlForArtifact, nil
}

func (c *Client) GenerateUnsignedGetUrlForArtifact(workflowId string, artifactType ArtifactType) (GenerateUnsignedGetUrlForArtifactResponse, error) {
	const mutation = `
mutation GenerateUnsignedGetUrlForArtifact($artifact: GenerateUnsignedGetUrlRequest!) {
  generateUnsignedGetUrlForArtifact(artifact: $artifact) {
    unsignedGetUrl
  }
}`
	req := graphql.NewRequest(mutation)
	reqVariables := map[string]any{
		"workflowId":              workflowId,
		"artifactType":            artifactType,
		"workflowRegistryAddress": c.workflowRegistryAddress,
		"chainSelector":           fmt.Sprintf("%v", c.chainSelector),
	}
	req.Var("artifact", reqVariables)

	var container struct {
		GenerateUnsignedGetUrlForArtifact GenerateUnsignedGetUrlForArtifactResponse `json:"generateUnsignedGetUrlForArtifact"`
	}

	ctx, cancel := c.CreateServiceContextWithTimeout()
	defer cancel()

	if err := c.graphql.
		Execute(ctx, req, &container); err != nil {
		return GenerateUnsignedGetUrlForArtifactResponse{}, err
	}

	c.log.Debug().Interface("response", container).
		Msg("Received GraphQL response from generateUnsignedGetUrlForArtifact")

	return container.GenerateUnsignedGetUrlForArtifact, nil
}

func calculateContentHash(content []byte) string {
	hash := md5.Sum(content)                                  //nolint:gosec
	contentHash := base64.StdEncoding.EncodeToString(hash[:]) // Convert to base64 string
	return contentHash
}

func (c *Client) UploadToOrigin(g GeneratePresignedPostUrlForArtifactResponse, content []byte, contentType string) error {
	c.log.Debug().Str("URL", g.PresignedPostURL).Msg("Uploading content to origin")

	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	// Add the presigned form fields to the request (do not add extra fields).
	for _, field := range g.PresignedPostFields {
		if err := w.WriteField(field.Key, field.Value); err != nil {
			c.log.Error().Err(err).Str("field", field.Key).Msg("Failed to write presigned field")
			return err
		}
	}

	contentHash := calculateContentHash(content)

	// Add the Content-Type header to the request.
	err := w.WriteField("Content-Type", contentType)
	if err != nil {
		return err
	}
	// Add the Content-MD5 header to the request.
	err = w.WriteField("Content-MD5", contentHash)
	if err != nil {
		return err
	}

	// Add the file to the request as the last field.
	fileWriter, err := w.CreateFormFile("file", "artifact")
	if err != nil {
		c.log.Error().Err(err).Msg("Failed to create form file field")
		return err
	}
	if _, err := fileWriter.Write(content); err != nil {
		c.log.Error().Err(err).Msg("Failed to write file content to form")
		return err
	}

	if err := w.Close(); err != nil {
		c.log.Error().Err(err).Msg("Failed to close multipart writer")
		return err
	}

	ctx, cancel := c.CreateHttpContextWithTimeout()
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, "POST", g.PresignedPostURL, &b)
	if err != nil {
		c.log.Error().Err(err).Msg("Failed to create HTTP request")
		return err
	}
	httpReq.Header.Set("Content-Type", w.FormDataContentType())

	httpClient := &http.Client{Timeout: c.httpTimeout}
	httpResp, err := httpClient.Do(httpReq) // #nosec G704 -- URL is from trusted CLI configuration
	if err != nil {
		c.log.Error().Err(err).Msg("HTTP request to origin failed")
		return err
	}
	defer func() {
		if cerr := httpResp.Body.Close(); cerr != nil {
			c.log.Warn().Err(cerr).Msg("Failed to close origin response body")
		}
	}()

	// Accept 204 No Content or 201 Created as success.
	if httpResp.StatusCode != http.StatusNoContent && httpResp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(httpResp.Body)
		c.log.Error().Int("status", httpResp.StatusCode).Str("body", string(body)).Msg("artifact upload failed")
		return fmt.Errorf("expected status 204 or 201, got %d: %s", httpResp.StatusCode, string(body))
	}

	c.log.Debug().Msg("Successfully uploaded content to origin")
	return nil
}

func (c *Client) UploadArtifactWithRetriesAndGetURL(
	workflowID string,
	artifactType ArtifactType,
	content []byte,
	contentType string) (GenerateUnsignedGetUrlForArtifactResponse, error) {
	if len(workflowID) == 0 {
		return GenerateUnsignedGetUrlForArtifactResponse{}, fmt.Errorf("workflowID is empty")
	}
	if len(content) == 0 {
		return GenerateUnsignedGetUrlForArtifactResponse{}, fmt.Errorf("content is empty for artifactType %s", artifactType)
	}

	c.log.Debug().Str("workflowID", workflowID).
		Str("artifactType", string(artifactType)).
		Msg("Generating presigned post URL for artifact")

	var g GeneratePresignedPostUrlForArtifactResponse
	shouldUpload := true
	err := retry.Do(
		func() error {
			var err error
			g, err = c.GeneratePostUrlForArtifact(workflowID, artifactType, content)
			if err != nil {
				if strings.Contains(err.Error(), "already exists") {
					shouldUpload = false
					c.log.Debug().Msg("Workflow artifact already exists, skipping upload.")
				} else {
					return fmt.Errorf("generate presigned post url: %w", err)
				}
			}
			return nil
		},
		retry.Attempts(3),
		retry.LastErrorOnly(true),
	)
	if err != nil {
		c.log.Error().Err(err).Msg("Failed to generate presigned post URL for artifact")
		return GenerateUnsignedGetUrlForArtifactResponse{}, err
	}

	c.log.Debug().Str("presignedPostURL", g.PresignedPostURL).
		Msg("Generated presigned post URL for artifact")

	if shouldUpload {
		err = retry.Do(
			func() error {
				return c.UploadToOrigin(g, content, contentType)
			},
			retry.Attempts(3),
			retry.LastErrorOnly(true),
		)
		if err != nil {
			c.log.Error().Err(err).Msg("Failed to upload content to origin")
			return GenerateUnsignedGetUrlForArtifactResponse{}, fmt.Errorf("upload to origin: %w", err)
		}
	}

	var g2 GenerateUnsignedGetUrlForArtifactResponse
	err = retry.Do(
		func() error {
			g2, err = c.GenerateUnsignedGetUrlForArtifact(workflowID, artifactType)
			if err != nil {
				return fmt.Errorf("generate unsigned get url: %w", err)
			}
			return nil
		},
		retry.Attempts(3),
		retry.LastErrorOnly(true),
	)
	if err != nil {
		c.log.Error().Err(err).Msg("Failed to generate unsigned get URL for artifact")
		return GenerateUnsignedGetUrlForArtifactResponse{}, err
	}

	return g2, nil
}
