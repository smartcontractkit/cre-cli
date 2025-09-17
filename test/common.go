package test

import (
	"errors"
	"fmt"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/dev-platform/internal/constants"
	"github.com/smartcontractkit/dev-platform/internal/settings"
)

var (
	L *zerolog.Logger
)

const (
	TestLogLevelEnvVar = "TEST_LOG_LEVEL" // export this env var before running tests if DEBUG level is needed
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
	ProposalDirectory    string
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
		ProposalDirectory:    fmt.Sprintf("/tmp/%s/", uid),
		ProjectDirectory:     fmt.Sprintf("/tmp/%s/", uid),
	}
	L.Info().Str("Test", t.Name()).Str("uid", uid).Interface("Config", config).Msg("Created test config")
	return &config
}

func (tc *TestConfig) GetCliEnvFlag() string {
	return fmt.Sprintf("--%s=%s", settings.Flags.CliEnvFile.Name, tc.EnvFile)
}

func (tc *TestConfig) GetCliSettingsFlag() string {
	return fmt.Sprintf("--%s=%s", settings.Flags.CliSettingsFile.Name, tc.WorkflowSettingsFile)
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

// Boot Anvil by either loading Anvil state or running a fresh instance that will dump its state on exit
// Input parameter can be LOAD_ANVIL_STATE=true or DUMP_ANVIL_STATE=false (look at the defined constants)
func StartAnvil(initState AnvilInitState) (*os.Process, int, error) {
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
	if initState == LOAD_ANVIL_STATE {
		// booting up Anvil with pre-baked contracts, required for some E2E tests
		args = append(args, "--load-state", "anvil-state.json")
	} else if initState == DUMP_ANVIL_STATE {
		// start fresh instance of Anvil, then deploy and configure contracts to bake them into the state dump
		args = append(args, "--dump-state", "anvil-state.json")
	} else {
		return nil, 0, errors.New("unknown anvil init state enum")
	}
	args = append(args, "--port", strconv.Itoa(port))

	anvil := exec.Command("anvil", args...)

	L.Info().Str("Command", anvil.String()).Msg("Executing anvil")
	if err := anvil.Start(); err != nil {
		return nil, 0, errors.New("failed to start Anvil")
	}

	L.Info().Msg("Checking if Anvil is up and running")
	for i := 0; i < 100; i++ { // limit retries to 10 seconds
		conn, err := net.DialTimeout("tcp", "localhost:"+strconv.Itoa(port), 1*time.Second)
		if err == nil {
			conn.Close()
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	L.Info().Msg("Anvil is running...")
	return anvil.Process, port, nil
}

func StopAnvil(anvilProc *os.Process) {
	if err := anvilProc.Kill(); err != nil {
		L.Err(err).Msg("Failed to kill Anvil")
	}
}
