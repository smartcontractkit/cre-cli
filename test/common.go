package test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/settings"
)

var (
	L *zerolog.Logger
)

// CLI path for testing (also defined in multi_command_flows for their use)
var CLIPath = os.TempDir() + string(os.PathSeparator) + "cre" + func() string {
	if os.PathSeparator == '\\' {
		return ".exe"
	}
	return ""
}()

const (
	TestLogLevelEnvVar = "TEST_LOG_LEVEL" // export this env var before running tests if DEBUG level is needed
	SethConfigPath     = "seth.toml"
	SettingsTarget     = "staging-settings"
)

// needed for StartAnvil() function, describes how to boot Anvil
type AnvilInitState int

const (
	LOAD_ANVIL_STATE AnvilInitState = iota
	DUMP_ANVIL_STATE AnvilInitState = iota
)

func InitLogging() {
	lvlStr := os.Getenv(TestLogLevelEnvVar)
	if lvlStr == "" {
		lvlStr = "info"
	}
	lvl, err := zerolog.ParseLevel(lvlStr)
	if err != nil {
		panic(err)
	}
	l := log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).Level(lvl)
	L = &l
}

type TestConfig struct {
	uid                  string
	EnvFile              string
	WorkflowSettingsFile string
	ProjectDirectory     string
}

func NewTestConfig(t *testing.T) *TestConfig {
	uid := "test-" + uuid.New().String()
	err := os.MkdirAll(fmt.Sprintf("/tmp/%s", uid), 0755)
	if err != nil {
		require.NoError(t, err, "Failed to create temporary directory")
	}
	config := TestConfig{
		uid:                  uid,
		EnvFile:              fmt.Sprintf("/tmp/%s/.env", uid),
		WorkflowSettingsFile: fmt.Sprintf("/tmp/%s/%s", uid, constants.DefaultWorkflowSettingsFileName),
		ProjectDirectory:     fmt.Sprintf("/tmp/%s/", uid),
	}
	L.Info().Str("Test", t.Name()).Str("uid", uid).Interface("Config", config).Msg("Created test config")
	return &config
}

func (tc *TestConfig) GetCliEnvFlag() string {
	return fmt.Sprintf("--%s=%s", settings.Flags.CliEnvFile.Name, tc.EnvFile)
}

func (tc *TestConfig) GetProjectRootFlag() string {
	return fmt.Sprintf("--%s=%s", settings.Flags.ProjectRoot.Name, tc.ProjectDirectory)
}

func (tc *TestConfig) Cleanup(t *testing.T) func() {
	return func() {
		if !t.Failed() {
			L.Info().Str("Test", t.Name()).Str("uid", tc.uid).Msg("Test passed, cleaning up")
			os.RemoveAll(fmt.Sprintf("/tmp/%s", tc.uid))
		} else {
			L.Warn().Str("Test", t.Name()).Str("uid", tc.uid).Msg("Test failed, keeping files for inspection")
		}
	}
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

func copyPath(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return copyDir(src, dst)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		_ = os.RemoveAll(dst) // replace if exists
		return os.Symlink(target, dst)
	}
	return copyFile(src, dst)
}

func copyDir(src, dst string) error {
	// Create the root destination directory with same perms
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, info.Mode().Perm()); err != nil {
		return err
	}

	return filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		// Skip the root (already created)
		if rel == "." {
			return nil
		}

		switch {
		case d.IsDir():
			fi, err := d.Info()
			if err != nil {
				return err
			}
			return os.MkdirAll(target, fi.Mode().Perm())

		case d.Type()&os.ModeSymlink != 0:
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return err
			}
			_ = os.RemoveAll(target)
			return os.Symlink(linkTarget, target)

		default:
			return copyFile(path, target)
		}
	})
}

// Boot Anvil by either loading Anvil state or running a fresh instance that will dump its state on exit
// Input parameter can be LOAD_ANVIL_STATE=true or DUMP_ANVIL_STATE=false (look at the defined constants)
func StartAnvil(initState AnvilInitState, stateFileName string) (*os.Process, int, error) {
	// introduce random delay to prevent tests binding to the same port for Anvil
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	minDelay := 1 * time.Millisecond
	maxDelay := 1000 * time.Millisecond
	randomDelay := time.Duration(r.Intn(int(maxDelay-minDelay))) + minDelay
	time.Sleep(randomDelay)

	L.Info().Msg("Booting up Anvil")
	// find an available port
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, 0, errors.New("failed to find an available port")
	}
	port := listener.Addr().(*net.TCPAddr).Port
	err = listener.Close()
	if err != nil {
		return nil, 0, errors.New("failed to close listener")
	}
	args := []string{"--chain-id", "31337"}
	switch initState {
	case LOAD_ANVIL_STATE:
		// booting up Anvil with pre-baked contracts, required for some E2E tests
		args = append(args, "--load-state", stateFileName)
	case DUMP_ANVIL_STATE:
		// start fresh instance of Anvil, then deploy and configure contracts to bake them into the state dump
		args = append(args, "--dump-state", stateFileName)
	default:
		return nil, 0, errors.New("unknown anvil init state enum")
	}
	args = append(args, "--port", strconv.Itoa(port))

	anvil := exec.Command("anvil", args...)

	var outBuf, errBuf bytes.Buffer
	anvil.Stdout = &outBuf
	anvil.Stderr = &errBuf

	L.Info().Str("Command", anvil.String()).Msg("Executing anvil")
	if err := anvil.Start(); err != nil {
		return nil, 0, fmt.Errorf("failed to start Anvil: %w", err)
	}

	L.Info().Msg("Checking if Anvil is up and running")

	anvilUp := false
	for i := 0; i < 100; i++ { // limit retries to 10 seconds
		conn, err := net.DialTimeout("tcp", "localhost:"+strconv.Itoa(port), 1*time.Second)
		if err == nil {
			anvilUp = true
			conn.Close()
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !anvilUp {
		L.Error().Str("Stdout", outBuf.String()).Str("Stderr", errBuf.String()).Msg("Anvil failed to start")
		return nil, 0, errors.New("anvil failed to start within the expected time")
	}

	L.Info().Msg("Anvil is running...")
	L.Debug().Str("Stdout", outBuf.String()).Str("Stderr", errBuf.String()).Msg("Anvil logs")
	return anvil.Process, port, nil
}

func StopAnvil(anvilProc *os.Process) {
	if err := anvilProc.Kill(); err != nil {
		L.Err(err).Msg("Failed to kill Anvil")
	}
}
