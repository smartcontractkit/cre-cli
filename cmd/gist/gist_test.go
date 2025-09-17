package gist

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsValidGistID(t *testing.T) {
	tests := []struct {
		gistID  string
		isValid bool
	}{
		{"1234567890abcdef1234567890abcdef", true},   // valid 32-character hex string
		{"1234567890abcdef1234567890abcdeg", false},  // contains invalid character 'g'
		{"1234567890abcdef1234567890abcde", false},   // less than 32 characters
		{"1234567890abcdef1234567890abcdef0", false}, // more than 32 characters
		{"", false}, // empty string
		{"1234567890ABCDEF1234567890ABCDEF", false}, // contains uppercase characters
	}

	for _, test := range tests {
		t.Run(test.gistID, func(t *testing.T) {
			if IsValidGistID(test.gistID) != test.isValid {
				t.Errorf("For gistID '%s', expected %v but got %v", test.gistID, test.isValid, !test.isValid)
			}
		})
	}
}

func TestGetURLWithoutRevision(t *testing.T) {
	inputURL := "https://gist.githubusercontent.com/username/2bc7815a1ca5a22ddda1b796772d024f/raw/54ded77798b87fc90667718c35e83544072eedc8/config.yaml"
	expectedOutput := "https://gist.githubusercontent.com/username/2bc7815a1ca5a22ddda1b796772d024f/raw"

	output, err := GetURLWithoutRevision(inputURL)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if output != expectedOutput {
		t.Errorf("Expected %s, but got %s", expectedOutput, output)
	}
}

func TestFetch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.WriteString(w, "test data")
		if err != nil {
			t.Fatalf("Failed to write response: %v", err)
		}
	}))
	defer server.Close()

	url := server.URL
	maxUrlLength := 100
	maxDataSize := 20

	data, err := Fetch(url, maxUrlLength, maxDataSize)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	expectedData := "test data"
	if string(data) != expectedData {
		t.Errorf("Expected data %q, got %q", expectedData, data)
	}
}

func TestFetch_MaxURLLengthExceeded(t *testing.T) {
	// Start a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.WriteString(w, "test data")
		if err != nil {
			t.Fatalf("Failed to write response: %v", err)
		}
	}))
	defer server.Close()

	// Create a URL that exceeds the max URL length
	longURL := server.URL + strings.Repeat("a", 100)
	maxUrlLength := len(server.URL) // Set max URL length to the server URL length

	_, err := Fetch(longURL, maxUrlLength, 20)
	if err == nil {
		t.Fatalf("Expected error due to exceeded URL length, got nil")
	}

	expectedError := fmt.Sprintf("URL length exceeds maximum size: %d", maxUrlLength)
	if err.Error() != expectedError {
		t.Errorf("Expected error message %q, got %q", expectedError, err.Error())
	}
}

func TestFetch_MaxDataSizeExceeded(t *testing.T) {
	// Start a test server with data exceeding the max data size
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.WriteString(w, strings.Repeat("x", 50)) // Return 50 bytes of data
		if err != nil {
			t.Fatalf("Failed to write response: %v", err)
		}
	}))
	defer server.Close()

	url := server.URL
	maxUrlLength := 100
	maxDataSize := 20 // Set max data size less than the server's response

	_, err := Fetch(url, maxUrlLength, maxDataSize)
	if err == nil {
		t.Fatalf("Expected error due to exceeded data size, got nil")
	}

	expectedError := fmt.Sprintf("data size of: 50 exceeds maximum size: %d", maxDataSize)
	if err.Error() != expectedError {
		t.Errorf("Expected error message %q, got %q", expectedError, err.Error())
	}
}

func TestCreateGistRequestBody(t *testing.T) {
	tests := []struct {
		description string
		public      bool
		files       map[string]GistFile
		expected    GistRequest
	}{
		{
			description: "wasm and config",
			public:      true,
			files: map[string]GistFile{
				"wasm":   {Content: "This is the wasm binary"},
				"config": {Content: "This is the config"},
			},
			expected: GistRequest{
				Description: "wasm and config",
				Public:      true,
				Files: map[string]GistFile{
					"wasm":   {Content: "This is the wasm binary"},
					"config": {Content: "This is the config"},
				},
			},
		},
		{
			description: "Empty Files",
			public:      false,
			files:       map[string]GistFile{},
			expected: GistRequest{
				Description: "Empty Files",
				Public:      false,
				Files:       map[string]GistFile{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			reqBody, err := createGistRequestBody(tt.description, tt.public, tt.files)

			assert.NoError(t, err)
			var result GistRequest
			err = json.Unmarshal(reqBody, &result)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractGistURL(t *testing.T) {
	tests := []struct {
		name        string
		respBody    GistResponse
		filePath    string
		expectedURL string
		shouldError bool
	}{
		{
			name: "Valid response with matching file path",
			respBody: GistResponse{
				Files: map[string]struct {
					RawURL   string `json:"raw_url"`
					FileName string `json:"filename"`
					Type     string `json:"type"`
					Encoding string `json:"encoding"`
				}{
					"file1.txt": {RawURL: "https://gist.github.com/raw/file1.txt", FileName: "file1.txt", Type: "text/plain"},
				},
			},
			filePath:    "path/to/file1.txt",
			expectedURL: "https://gist.github.com/raw/file1.txt",
			shouldError: false,
		},
		{
			name: "Valid response without matching file path",
			respBody: GistResponse{
				Files: map[string]struct {
					RawURL   string `json:"raw_url"`
					FileName string `json:"filename"`
					Type     string `json:"type"`
					Encoding string `json:"encoding"`
				}{
					"file2.txt": {RawURL: "https://gist.github.com/raw/file2.txt", FileName: "file2.txt", Type: "text/plain"},
				},
			},
			filePath:    "path/to/file1.txt",
			expectedURL: "",
			shouldError: true,
		},
		{
			name:        "Invalid JSON response",
			respBody:    GistResponse{},
			filePath:    "file1.json",
			expectedURL: "",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			respBodyBytes, err := json.Marshal(tt.respBody)
			assert.NoError(t, err)

			url, err := extractGistURL(respBodyBytes, tt.filePath)
			if tt.shouldError {
				assert.Error(t, err)
				assert.Empty(t, url)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedURL, url)
			}
		})
	}
}

func TestPrepareGistFile(t *testing.T) {
	tests := []struct {
		name        string
		filePath    string
		fileContent string
		expectedMap map[string]GistFile
		shouldError bool
	}{
		{
			name:        "Valid file path",
			filePath:    "testfile.txt",
			fileContent: "This is a test file content",
			expectedMap: map[string]GistFile{
				"testfile.txt": {Content: "This is a test file content"},
			},
			shouldError: false,
		},
		{
			name:        "Invalid file path",
			filePath:    "nonexistent.txt",
			fileContent: "",
			expectedMap: nil,
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.shouldError {
				// Create a temporary file for testing
				err := os.WriteFile(tt.filePath, []byte(tt.fileContent), 0600)
				assert.NoError(t, err)
				defer os.Remove(tt.filePath) // Clean up after the test
			}

			result, err := prepareGistFile(tt.filePath, false)
			if tt.shouldError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedMap, result)
			}
		})
	}
}

func TestMakeGistHTTPRequest(t *testing.T) {
	tests := []struct {
		name         string
		method       string
		body         []byte
		token        GitHubAPIToken
		statusCode   int
		responseBody string
		expectError  bool
	}{
		{
			name:         "Successful POST request",
			method:       http.MethodPost,
			body:         []byte(`{"test": "data"}`),
			token:        GitHubAPIToken("test-token"),
			statusCode:   http.StatusCreated,
			responseBody: `{"success": true}`,
			expectError:  false,
		},
		{
			name:         "Request fails with 400 status code",
			method:       http.MethodPost,
			body:         []byte(`{"test": "data"}`),
			token:        GitHubAPIToken("test-token"),
			statusCode:   http.StatusBadRequest,
			responseBody: `{"error": "bad request"}`,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock HTTP server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, tt.method, r.Method)
				assert.Equal(t, "token "+tt.token.RawValue(), r.Header.Get("Authorization"))
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

				w.WriteHeader(tt.statusCode)
				_, err := w.Write([]byte(tt.responseBody))
				assert.NoError(t, err)
			}))
			defer server.Close()

			// Make the HTTP request
			respBody, err := makeGistHTTPRequest(tt.method, tt.body, server.URL, tt.token)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, respBody)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, respBody)
				assert.Equal(t, tt.responseBody, string(respBody))
			}
		})
	}
}
