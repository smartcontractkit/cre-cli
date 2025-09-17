package gist

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rs/zerolog"

	cmdCommon "github.com/smartcontractkit/cre-cli/cmd/common"
)

const (
	GITHUB_GIST_API_URL = "https://api.github.com/gists"
	GITHUB_USER_API_URL = "https://api.github.com/user"
)

type GistFile struct {
	Content string `json:"content"`
}

type GistRequest struct {
	Description string              `json:"description"`
	Public      bool                `json:"public"`
	Files       map[string]GistFile `json:"files"`
}

type GistResponse struct {
	Files map[string]struct {
		RawURL   string `json:"raw_url"`
		FileName string `json:"filename"`
		Type     string `json:"type"`
		Encoding string `json:"encoding"`
	} `json:"files"`
	GitPullURL string `json:"git_pull_url"`
}

type GitHubUser struct {
	Login string `json:"login"` // GitHub username
}

func CheckGitHubTokenGistPermissions(token GitHubAPIToken) (bool, error) {
	// request goes to "List gists for the authenticated user" API
	req, err := http.NewRequest("GET", GITHUB_GIST_API_URL, nil)
	if err != nil {
		return false, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token.RawValue())
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	// it's not straightforward and easy to check read and write permissions for GitHub PAT tokens yet
	// if this token is able to list all Gists for an authorized user, then it has Gist read permissions
	// we could try to create a test Gist and then delete it, but that may end up with undesired consequences
	// for the user if the command is terminated between these two actions (test Gist is never deleted)
	return resp.StatusCode == http.StatusOK, nil
}

func CreateGistForTextualContent(l *zerolog.Logger, description string, filePath string, isPublic bool, token GitHubAPIToken) (string, error) {
	gistFile, err := prepareGistFile(filePath, false)
	if err != nil {
		return "", err
	}

	return sendGistCreateRequest(l, description, filePath, gistFile, isPublic, token)
}

// CreateGistForDirectory creates a Gist for the directory containing the source code files. It will only include Go files and go.mod.
// It will return Gist URL for the provided file path where the main function resides.
func CreateGistForDirectory(l *zerolog.Logger, description string, workflowPath string, isPublic bool, token GitHubAPIToken) (string, error) {
	dir := filepath.Dir(workflowPath)
	gistFile, err := prepareGistFileForSourceCodeDirectory(dir)
	if err != nil {
		return "", err
	}

	return sendGistCreateRequestForDirectory(l, description, gistFile, isPublic, token)
}

func CreateGistForBinaryContent(l *zerolog.Logger, description string, filePath string, isPublic bool, token GitHubAPIToken) (string, error) {
	gistFile, err := prepareGistFile(filePath, true)
	if err != nil {
		return "", err
	}

	return sendGistCreateRequest(l, description, filePath, gistFile, isPublic, token)
}

func UpdateGistForTextualContent(l *zerolog.Logger, gistID string, description string, filePath string, isPublic bool, token GitHubAPIToken) (string, error) {
	gistFile, err := prepareGistFile(filePath, false)
	if err != nil {
		return "", err
	}

	return sendGistUpdateRequest(l, gistID, description, filePath, gistFile, isPublic, token)
}

func UpdateGistForBinaryContent(l *zerolog.Logger, gistID string, description string, filePath string, isPublic bool, token GitHubAPIToken) (string, error) {
	gistFile, err := prepareGistFile(filePath, true)
	if err != nil {
		return "", err
	}

	return sendGistUpdateRequest(l, gistID, description, filePath, gistFile, isPublic, token)
}

func Fetch(url string, maxUrlLength int, maxDataSize int) ([]byte, error) {
	if len(url) > maxUrlLength {
		return nil, fmt.Errorf("URL length exceeds maximum size: %d", maxUrlLength)
	}

	resp, err := http.Get(url) //nolint
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	// Check HTTP status code, we accept only 200
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch URL: status code %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if len(data) > maxDataSize {
		return nil, fmt.Errorf("data size of: %d exceeds maximum size: %d", len(data), maxDataSize)
	}

	return data, nil
}

func IsValidGistID(gistID string) bool {
	// Gist ID must be a 32-character string with only hex characters (0-9, a-f)
	match, _ := regexp.MatchString("^[0-9a-f]{32}$", gistID)
	return match
}

func GetURLWithoutRevision(revisionGistURL string) (string, error) {
	parsedURL, err := url.Parse(revisionGistURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse Gist URL: %w", err)
	}
	// Remove the last two segments from the path
	segments := strings.Split(parsedURL.Path, "/")
	if len(segments) < 3 {
		return "", fmt.Errorf("failed to extract top level Gist URL from %s", revisionGistURL)
	}
	parsedURL.Path = strings.Join(segments[:3], "/")
	parsedURL.Path += "/raw"
	finalURL := parsedURL.String()
	return finalURL, nil
}

func IsGistBinaryContent(l *zerolog.Logger, gistID string, token GitHubAPIToken) (bool, error) {
	url := fmt.Sprintf("%s/%s", GITHUB_GIST_API_URL, gistID)
	respBody, err := makeGistHTTPRequest("GET", []byte{}, url, token)
	if err != nil {
		return false, fmt.Errorf("failed to perform HTTP request to update Gist: %w", err)
	}

	var result GistResponse
	err = json.Unmarshal(respBody, &result)
	if err != nil {
		return false, fmt.Errorf("failed to parse response from Gist: %w", err)
	}

	if len(result.Files) != 1 {
		return false, errors.New("gist does not contain a single file as expected")
	}

	for _, properties := range result.Files {
		return cmdCommon.IsBinaryFile(properties.FileName)
	}

	return false, errors.New("not able to detect file extension of uploaded file to Gist")
}

func prepareGistFile(filePath string, isBinary bool) (map[string]GistFile, error) {
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	var payload string
	if isBinary {
		// always apply base64 encoding to binary data so it doesn't get corrupted during JSON marshalling or network transfer
		payload = base64.StdEncoding.EncodeToString(fileContent)
	} else {
		payload = string(fileContent)
	}

	files := map[string]GistFile{
		filepath.Base(filePath): {Content: payload},
	}

	return files, nil
}

// prepareGistFileForSourceCodeDirectory prepares Gist files for the source code directory. It will only include Go files and go.mod.
func prepareGistFileForSourceCodeDirectory(dir string) (map[string]GistFile, error) {
	files := make(map[string]GistFile)

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// only include Go files and go.mod
		if info.IsDir() || (info.Name() != "go.mod" && !strings.HasSuffix(info.Name(), ".go")) {
			return nil
		}

		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		// a temporary workaround to replace forward slashes with backslashes
		// because Gist does not allow subdirectory paths in the file names
		relPath = strings.ReplaceAll(relPath, "/", "\\")

		fileContent, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		files[relPath] = GistFile{Content: string(fileContent)}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to prepare Gist files for directory %s: %w", dir, err)
	}

	return files, nil
}

func sendGistCreateRequestForDirectory(l *zerolog.Logger, description string, gistFile map[string]GistFile, isPublic bool, token GitHubAPIToken) (string, error) {
	reqBody, err := createGistRequestBody(description, isPublic, gistFile)
	if err != nil {
		return "", err
	}

	l.Debug().Msg("Sending request to create a new Gist...")

	respBody, err := makeGistHTTPRequest("POST", reqBody, GITHUB_GIST_API_URL, token)
	if err != nil {
		return "", fmt.Errorf("failed to perform HTTP request to create Gist: %w", err)
	}

	var result GistResponse
	err = json.Unmarshal(respBody, &result)
	if err != nil {
		return "", fmt.Errorf("failed to parse response from Gist: %w", err)
	}

	return result.GitPullURL, nil
}

func sendGistCreateRequest(l *zerolog.Logger, description string, filePath string, gistFile map[string]GistFile, isPublic bool, token GitHubAPIToken) (string, error) {
	reqBody, err := createGistRequestBody(description, isPublic, gistFile)
	if err != nil {
		return "", err
	}

	l.Debug().Msg("Sending request to create a new Gist...")

	respBody, err := makeGistHTTPRequest("POST", reqBody, GITHUB_GIST_API_URL, token)
	if err != nil {
		return "", fmt.Errorf("failed to perform HTTP request to create Gist: %w", err)
	}

	return extractGistURL(respBody, filePath)
}

func sendGistUpdateRequest(l *zerolog.Logger, gistID string, description string, filePath string, gistFile map[string]GistFile, isPublic bool, token GitHubAPIToken) (string, error) {
	reqBody, err := createGistRequestBody(description, isPublic, gistFile)
	if err != nil {
		return "", err
	}

	l.Debug().Msg("Updating Gist...")

	url := fmt.Sprintf("%s/%s", GITHUB_GIST_API_URL, gistID)
	respBody, err := makeGistHTTPRequest("PATCH", reqBody, url, token)
	if err != nil {
		return "", fmt.Errorf("failed to perform HTTP request to update Gist: %w", err)
	}

	return extractGistURL(respBody, filePath)
}

func extractGistURL(respBody []byte, filePath string) (string, error) {
	var result GistResponse
	err := json.Unmarshal(respBody, &result)
	if err != nil {
		return "", fmt.Errorf("failed to parse response from Gist: %w", err)
	}

	if fileProperties, ok := result.Files[filepath.Base(filePath)]; ok {
		return fileProperties.RawURL, nil
	}

	return "", fmt.Errorf("failed to extract Gist URL from response")
}

func createGistRequestBody(description string, public bool, files map[string]GistFile) ([]byte, error) {
	gistRequest := GistRequest{
		Description: description,
		Public:      public,
		Files:       files,
	}

	reqBody, err := json.Marshal(gistRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}
	return reqBody, nil
}

func makeGistHTTPRequest(method string, body []byte, url string, token GitHubAPIToken) ([]byte, error) {
	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	req.Header.Set("Authorization", "token "+token.RawValue())
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("request failed with status %s: %s", resp.Status, respBody)
	}

	return respBody, nil
}
