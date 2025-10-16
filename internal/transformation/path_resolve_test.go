package transformation

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePath(t *testing.T) {
	cwd, _ := os.Getwd()

	// Create a temporary file in the CWD
	tempFile := filepath.Join(cwd, "temp_test_file.txt")
	file, err := os.Create(tempFile)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	file.Close()
	defer os.Remove(tempFile) // Ensure cleanup

	// Create a temporary directory in the CWD
	tempDir := filepath.Join(cwd, "temp_test_dir")
	err = os.Mkdir(tempDir, os.ModePerm)
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir) // Ensure cleanup

	tests := []struct {
		name    string
		input   string
		expects string
		wantErr bool
	}{
		{"Empty input should return CWD", "", cwd, false},
		{"Existing file should return absolute path", filepath.Join("temp_test_file.txt"), filepath.Join(cwd, "temp_test_file.txt"), false},
		{"Existing directory should return absolute path", filepath.Join("temp_test_dir"), filepath.Join(cwd, "temp_test_dir"), false},
		{"Non-existing path should return error", filepath.Join("does_not_exist.txt"), "", true},
		{"Absolute non-existing path should return error", "/non/existing/path", "", true},
		{"Path with redundant slashes should be cleaned", "//test//path", filepath.Join(cwd, "test/path"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolvePath(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ResolvePath() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.expects && !tt.wantErr {
				t.Errorf("ResolvePath() = %v, want %v", got, tt.expects)
			}
		})
	}
}

func TestResolveDirectoryOrCreate(t *testing.T) {
	cwd, _ := os.Getwd()
	tempDir := filepath.Join(cwd, "test_temp_dir")
	defer os.RemoveAll(tempDir)

	// Create a temporary file to test that ResolveDirectoryOrCreate does not overwrite files
	tempFile := filepath.Join(tempDir, "existing_file.txt")
	err := os.MkdirAll(filepath.Dir(tempFile), os.ModePerm)
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	err = os.WriteFile(tempFile, []byte("test"), 0600)
	if err != nil {
		t.Fatalf("Failed to write file.: %v", err)
	}

	tests := []struct {
		name    string
		input   string
		expects string
		wantErr bool
	}{
		{"Empty input should return CWD", "", cwd, false},
		{"Should create directory", filepath.Join(tempDir, "new_folder"), filepath.Join(tempDir, "new_folder"), false},
		{"Already existing directory should return its path", tempDir, tempDir, false},
		{"Relative path should resolve and create", "relative_dir", filepath.Join(cwd, "relative_dir"), false},
		{"Deep nested directory should be created", filepath.Join(tempDir, "nested/dir/structure"), filepath.Join(tempDir, "nested/dir/structure"), false},
		{"Path with redundant slashes should be cleaned and created", filepath.Join(tempDir, "nested//dir"), filepath.Join(tempDir, "nested/dir"), false},
		{"Absolute path should resolve and create", "/tmp/test_abs_dir", "/tmp/test_abs_dir", false},
		{"File path should only create parent directory", filepath.Join(tempDir, "file_dir/test.txt"), filepath.Join(tempDir, "file_dir/test.txt"), false},
		{"Path with '..' should resolve correctly", filepath.Join(tempDir, "dir/../new_dir"), filepath.Join(tempDir, "new_dir"), false},
		{"Existing file should not be replaced by a directory", tempFile, tempFile, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveDirectoryOrCreate(tt.input)
			t.Logf("ResolveDirectoryOrCreate() returned: %v", got)
			if (err != nil) != tt.wantErr {
				t.Errorf("ResolveDirectoryOrCreate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.expects && !tt.wantErr {
				t.Errorf("ResolveDirectoryOrCreate() = %v, want %v", got, tt.expects)
			}
		})
	}
}

func TestResolveFilePathOrCreate(t *testing.T) {
	cwd, _ := os.Getwd()
	tempDir := filepath.Join(cwd, "test_temp_dir")
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name    string
		input   string
		expects string
		wantErr bool
	}{
		{"Empty input should return error", "", cwd, true},
		{"Should create file", filepath.Join(tempDir, "new_file.txt"), filepath.Join(tempDir, "new_file.txt"), false},
		{"Path with redundant slashes should be cleaned and file created", filepath.Join(tempDir, "logs///app.log"), filepath.Join(tempDir, "logs/app.log"), false},
		{"Nested file should create directories and file", filepath.Join(tempDir, "nested/dir/file.txt"), filepath.Join(tempDir, "nested/dir/file.txt"), false},
		{"File inside existing directory should be created", filepath.Join(tempDir, "existing_dir/existing_file.txt"), filepath.Join(tempDir, "existing_dir/existing_file.txt"), false},
		{"Absolute path should resolve and create file", "/tmp/test_file.txt", "/tmp/test_file.txt", false},
		{"Should create deep nested directories and file", filepath.Join(tempDir, "deep/nested/dir/structure/file.log"), filepath.Join(tempDir, "deep/nested/dir/structure/file.log"), false},
		{"Should create file within multiple missing directories", filepath.Join(tempDir, "multiple/levels/of/missing/directories/file.txt"), filepath.Join(tempDir, "multiple/levels/of/missing/directories/file.txt"), false},
		{"Should create file in existing directory", filepath.Join(tempDir, "existing_dir/another_file.txt"), filepath.Join(tempDir, "existing_dir/another_file.txt"), false},
		{"Should correctly handle absolute nested paths", "/tmp/deeply/nested/test_dir/test_file.log", "/tmp/deeply/nested/test_dir/test_file.log", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveFilePathOrCreate(tt.input)
			t.Logf("ResolveFilePathOrCreate() returned: %v", got)
			if (err != nil) != tt.wantErr {
				t.Errorf("ResolveFilePathOrCreate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.expects && !tt.wantErr {
				t.Errorf("ResolveFilePathOrCreate() = %v, want %v", got, tt.expects)
			}
		})
	}
}

func TestResolveWorkflowPath(t *testing.T) {
	t.Parallel()

	// Create a temporary directory for testing to avoid polluting project files
	tempDir, err := os.MkdirTemp("", "workflow_path_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create temporary workflow directories for testing
	workflowDir1 := filepath.Join(tempDir, "workflow1")
	workflowDir2 := filepath.Join(tempDir, "src", "workflow2")
	nestedWorkflowDir := filepath.Join(tempDir, "nested", "deep", "workflow3")

	// Create directories
	dirs := []string{workflowDir1, workflowDir2, nestedWorkflowDir}
	for _, dir := range dirs {
		err := os.MkdirAll(dir, os.ModePerm)
		if err != nil {
			t.Fatalf("Failed to create test directory %s: %v", dir, err)
		}
	}

	// Get absolute paths for expected results
	expectedWorkflowDir1, _ := filepath.Abs(workflowDir1)
	expectedWorkflowDir2, _ := filepath.Abs(workflowDir2)
	expectedNestedWorkflowDir, _ := filepath.Abs(nestedWorkflowDir)

	// Create a temporary file to test non-directory error
	tempFile := filepath.Join(tempDir, "test_file.txt")
	file, err := os.Create(tempFile)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	file.Close()

	tests := []struct {
		name    string
		input   string
		expects string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "Empty path should error",
			input:   "",
			expects: "",
			wantErr: true,
			errMsg:  "workflow path cannot be empty",
		},
		{
			name:    "Absolute path - workflow1",
			input:   workflowDir1,
			expects: expectedWorkflowDir1,
			wantErr: false,
		},
		{
			name:    "Absolute path - nested workflow",
			input:   workflowDir2,
			expects: expectedWorkflowDir2,
			wantErr: false,
		},
		{
			name:    "Absolute path - deep nested workflow",
			input:   nestedWorkflowDir,
			expects: expectedNestedWorkflowDir,
			wantErr: false,
		},
		{
			name:    "Non-existing absolute directory should error",
			input:   filepath.Join(tempDir, "nonexistent"),
			expects: "",
			wantErr: true,
			errMsg:  "workflow directory does not exist",
		},
		{
			name:    "File path (not directory) should error",
			input:   tempFile,
			expects: "",
			wantErr: true,
			errMsg:  "workflow path must be a directory",
		},
		{
			name:    "Path with redundant separators should be cleaned",
			input:   workflowDir1 + "//",
			expects: expectedWorkflowDir1,
			wantErr: false,
		},
		{
			name:    "Path with dot components should be cleaned",
			input:   filepath.Join(tempDir, "src", "..", "workflow1"),
			expects: expectedWorkflowDir1,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveWorkflowPath(tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("ResolveWorkflowPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("ResolveWorkflowPath() error message = %v, want %v", err.Error(), tt.errMsg)
				}
				return
			}

			if got != tt.expects {
				t.Errorf("ResolveWorkflowPath() = %v, want %v", got, tt.expects)
			}
		})
	}
}

func TestResolvePathRelativeTo(t *testing.T) {
	// Create a temp directory to use as base
	tempBase, err := os.MkdirTemp("", "test_base_dir_*")
	if err != nil {
		t.Fatalf("Failed to create temp base directory: %v", err)
	}
	defer os.RemoveAll(tempBase)

	// Create a subdirectory in the temp base
	subDir := filepath.Join(tempBase, "subdir")
	if err := os.Mkdir(subDir, os.ModePerm); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	// Create a test file
	testFile := filepath.Join(tempBase, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name    string
		path    string
		baseDir string
		want    string
		wantErr bool
	}{
		{
			name:    "Empty path returns empty string",
			path:    "",
			baseDir: tempBase,
			want:    "",
			wantErr: false,
		},
		{
			name:    "Absolute path returns unchanged",
			path:    "/absolute/path/to/file.json",
			baseDir: tempBase,
			want:    "/absolute/path/to/file.json",
			wantErr: false,
		},
		{
			name:    "Relative path with ./ prefix",
			path:    "./data.json",
			baseDir: tempBase,
			want:    filepath.Join(tempBase, "data.json"),
			wantErr: false,
		},
		{
			name:    "Relative path without prefix",
			path:    "data.json",
			baseDir: tempBase,
			want:    filepath.Join(tempBase, "data.json"),
			wantErr: false,
		},
		{
			name:    "Relative path with subdirectory",
			path:    "subdir/data.json",
			baseDir: tempBase,
			want:    filepath.Join(tempBase, "subdir", "data.json"),
			wantErr: false,
		},
		{
			name:    "Relative path with ../ parent reference",
			path:    "../sibling/data.json",
			baseDir: subDir,
			want:    filepath.Join(tempBase, "sibling", "data.json"),
			wantErr: false,
		},
		{
			name:    "Complex relative path",
			path:    "./foo/../bar/./baz.txt",
			baseDir: tempBase,
			want:    filepath.Join(tempBase, "bar", "baz.txt"),
			wantErr: false,
		},
		{
			name:    "Path with multiple slashes",
			path:    "foo//bar///file.json",
			baseDir: tempBase,
			want:    filepath.Join(tempBase, "foo", "bar", "file.json"),
			wantErr: false,
		},
		{
			name:    "Existing file path",
			path:    "test.txt",
			baseDir: tempBase,
			want:    testFile,
			wantErr: false,
		},
		{
			name:    "Non-existing file path (should still resolve)",
			path:    "nonexistent.json",
			baseDir: tempBase,
			want:    filepath.Join(tempBase, "nonexistent.json"),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolvePathRelativeTo(tt.path, tt.baseDir)
			if (err != nil) != tt.wantErr {
				t.Errorf("ResolvePathRelativeTo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Clean both paths for comparison (handles OS-specific path separators)
			gotCleaned := filepath.Clean(got)
			wantCleaned := filepath.Clean(tt.want)

			if gotCleaned != wantCleaned {
				t.Errorf("ResolvePathRelativeTo() = %v, want %v", gotCleaned, wantCleaned)
			}

			// For non-empty paths, verify the result is absolute
			if tt.path != "" && !filepath.IsAbs(got) && !tt.wantErr {
				t.Errorf("ResolvePathRelativeTo() returned non-absolute path: %v", got)
			}
		})
	}
}

func TestResolvePathRelativeTo_ErrorCases(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		baseDir string
		wantErr bool
	}{
		{
			name:    "Empty base directory with relative path",
			path:    "./data.json",
			baseDir: "",
			wantErr: false, // filepath.Join handles empty base as current dir
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ResolvePathRelativeTo(tt.path, tt.baseDir)
			if (err != nil) != tt.wantErr {
				t.Errorf("ResolvePathRelativeTo() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestResolvePathRelativeTo_WorkflowScenario(t *testing.T) {
	// Simulate the actual workflow scenario
	// Project structure:
	//   /project-root/
	//     data/payload.json
	//     workflows/my-workflow/
	//       workflow.yaml

	projectRoot, err := os.MkdirTemp("", "project_root_*")
	if err != nil {
		t.Fatalf("Failed to create project root: %v", err)
	}
	defer os.RemoveAll(projectRoot)

	// Create data directory and file
	dataDir := filepath.Join(projectRoot, "data")
	if err := os.Mkdir(dataDir, os.ModePerm); err != nil {
		t.Fatalf("Failed to create data directory: %v", err)
	}

	payloadFile := filepath.Join(dataDir, "payload.json")
	if err := os.WriteFile(payloadFile, []byte(`{"test": "data"}`), 0644); err != nil {
		t.Fatalf("Failed to create payload file: %v", err)
	}

	// Create workflows directory
	workflowsDir := filepath.Join(projectRoot, "workflows")
	if err := os.Mkdir(workflowsDir, os.ModePerm); err != nil {
		t.Fatalf("Failed to create workflows directory: %v", err)
	}

	workflowDir := filepath.Join(workflowsDir, "my-workflow")
	if err := os.Mkdir(workflowDir, os.ModePerm); err != nil {
		t.Fatalf("Failed to create workflow directory: %v", err)
	}

	tests := []struct {
		name         string
		userInput    string
		expectedPath string
	}{
		{
			name:         "Relative path from project root",
			userInput:    "./data/payload.json",
			expectedPath: payloadFile,
		},
		{
			name:         "Relative path without leading ./",
			userInput:    "data/payload.json",
			expectedPath: payloadFile,
		},
		{
			name:         "Absolute path unchanged",
			userInput:    payloadFile,
			expectedPath: payloadFile,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolvePathRelativeTo(tt.userInput, projectRoot)
			if err != nil {
				t.Errorf("ResolvePathRelativeTo() error = %v", err)
				return
			}

			if filepath.Clean(got) != filepath.Clean(tt.expectedPath) {
				t.Errorf("ResolvePathRelativeTo() = %v, want %v", got, tt.expectedPath)
			}

			if _, err := os.Stat(got); os.IsNotExist(err) {
				t.Errorf("Resolved path does not exist: %v", got)
			}
		})
	}
}
