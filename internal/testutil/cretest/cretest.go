package cretest

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/creconfig"
	"github.com/smartcontractkit/cre-cli/internal/testutil/testjwt"
)

const credentialsFile = "cre.yaml"

var cliBinary string

// SetCLIBinary sets the path to the cre CLI binary built for integration tests.
// Call from test.TestMain after building the binary.
func SetCLIBinary(path string) {
	cliBinary = path
}

// CLIBinary returns the integration-test CLI binary path.
func CLIBinary() string {
	return cliBinary
}

// Env holds an isolated CLI config directory for a test.
type Env struct {
	ConfigDir string
}

// NewEnv creates a temp config directory, sets CRE_CONFIG_DIR for the test process,
// and returns an Env for subprocess CLI runs in the same test.
func NewEnv(t *testing.T) *Env {
	t.Helper()
	dir := filepath.Join(t.TempDir(), creconfig.Dir)
	require.NoError(t, os.MkdirAll(dir, 0o700))
	t.Setenv(creconfig.ConfigDirEnvVar, dir)
	return &Env{ConfigDir: dir}
}

// IsolateConfig is an alias for NewEnv for in-process tests that write under ~/.cre.
func IsolateConfig(t *testing.T) string {
	t.Helper()
	return NewEnv(t).ConfigDir
}

// PinGoCacheForProcess keeps GOPATH/GOMODCACHE on the real user paths when tests
// override HOME or use temp directories.
func PinGoCacheForProcess(t *testing.T) {
	t.Helper()
	gopath, gomodcache := realGoCacheDirs(t)
	t.Setenv("GOPATH", gopath)
	t.Setenv("GOMODCACHE", gomodcache)
}

func realGoCacheDirs(t *testing.T) (gopath, gomodcache string) {
	t.Helper()
	realHome, err := os.UserHomeDir()
	require.NoError(t, err)

	gopath = os.Getenv("GOPATH")
	if gopath == "" {
		gopath = filepath.Join(realHome, "go")
	}
	gomodcache = os.Getenv("GOMODCACHE")
	if gomodcache == "" {
		gomodcache = filepath.Join(gopath, "pkg", "mod")
	}
	return gopath, gomodcache
}

// SeedBearerCredentials writes cre.yaml with a test JWT into configDir.
func SeedBearerCredentials(t *testing.T, configDir, orgID string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(configDir, 0o700))
	jwt := testjwt.CreateTestJWT(orgID)
	creConfig := "AccessToken: " + jwt + "\n" +
		"IDToken: test-id-token\n" +
		"RefreshToken: test-refresh-token\n" +
		"ExpiresIn: 3600\n" +
		"TokenType: Bearer\n"
	path := filepath.Join(configDir, credentialsFile)
	require.NoError(t, os.WriteFile(path, []byte(creConfig), 0o600))
}

// CLIEnv builds subprocess environment with isolated CRE_CONFIG_DIR and pinned Go caches.
func CLIEnv(t *testing.T, configDir string) []string {
	t.Helper()
	gopath, gomodcache := realGoCacheDirs(t)
	prefix := creconfig.ConfigDirEnvVar + "="

	childEnv := make([]string, 0, len(os.Environ())+3)
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, prefix) ||
			strings.HasPrefix(entry, "GOPATH=") ||
			strings.HasPrefix(entry, "GOMODCACHE=") {
			continue
		}
		childEnv = append(childEnv, entry)
	}
	childEnv = append(childEnv,
		creconfig.ConfigDirEnvVar+"="+configDir,
		"GOPATH="+gopath,
		"GOMODCACHE="+gomodcache,
	)
	if runtime.GOOS == "windows" {
		childEnv = append(childEnv, "USERPROFILE="+os.Getenv("USERPROFILE"))
	}
	return childEnv
}

func configDirForCLI(t *testing.T, env *Env) string {
	t.Helper()
	if env != nil && env.ConfigDir != "" {
		return env.ConfigDir
	}
	if dir := strings.TrimSpace(os.Getenv(creconfig.ConfigDirEnvVar)); dir != "" {
		return dir
	}
	return NewEnv(t).ConfigDir
}

// RunOption configures RunCLI.
type RunOption func(*runConfig)

type runConfig struct {
	env       *Env
	dir       string
	stdin     io.Reader
	bearerOrg string
	extraEnv  []string
}

// WithEnv uses an existing isolated config directory.
func WithEnv(env *Env) RunOption {
	return func(c *runConfig) { c.env = env }
}

// WithDir sets the subprocess working directory.
func WithDir(dir string) RunOption {
	return func(c *runConfig) { c.dir = dir }
}

// WithStdin sets subprocess stdin.
func WithStdin(r io.Reader) RunOption {
	return func(c *runConfig) { c.stdin = r }
}

// WithBearerCredentials seeds cre.yaml before running the CLI.
func WithBearerCredentials(orgID string) RunOption {
	return func(c *runConfig) { c.bearerOrg = orgID }
}

// WithExtraEnv appends additional KEY=value entries to the subprocess environment.
func WithExtraEnv(entries ...string) RunOption {
	return func(c *runConfig) { c.extraEnv = append(c.extraEnv, entries...) }
}

// Result holds CLI subprocess output.
type Result struct {
	Stdout string
	Stderr string
}

// Combined returns stdout and stderr concatenated.
func (r Result) Combined() string {
	return r.Stdout + r.Stderr
}

// RunCLI runs the cre binary with isolated CRE_CONFIG_DIR. binary may be empty to use SetCLIBinary path.
func RunCLI(t *testing.T, binary string, args []string, opts ...RunOption) (Result, error) {
	t.Helper()
	if binary == "" {
		binary = cliBinary
	}
	require.NotEmpty(t, binary, "cretest: CLI binary path not set; call cretest.SetCLIBinary in TestMain")

	var cfg runConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	configDir := configDirForCLI(t, cfg.env)
	if cfg.bearerOrg != "" {
		SeedBearerCredentials(t, configDir, cfg.bearerOrg)
	}

	cmd := exec.Command(binary, args...)
	cmd.Env = CLIEnv(t, configDir)
	if len(cfg.extraEnv) > 0 {
		cmd.Env = append(cmd.Env, cfg.extraEnv...)
	}
	if cfg.dir != "" {
		cmd.Dir = cfg.dir
	}
	if cfg.stdin != nil {
		cmd.Stdin = cfg.stdin
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return Result{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}, err
}
