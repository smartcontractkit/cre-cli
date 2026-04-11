package deploy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/client/privateregistryclient"
	"github.com/smartcontractkit/cre-cli/internal/testutil/chainsim"
)

func TestBuildPrivateRegistryInput(t *testing.T) {
	t.Parallel()

	t.Run("maps required fields and sets active status", func(t *testing.T) {
		t.Parallel()
		simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)
		defer simulatedEnvironment.Close()

		ctx, buf := simulatedEnvironment.NewRuntimeContextWithBufferedOutput()
		h := newHandler(ctx, buf)
		h.inputs = Inputs{
			WorkflowName: "my-workflow",
			BinaryURL:    "https://storage.example.com/binary.wasm",
			DonFamily:    "zone-a",
			WorkflowTag:  "v1-tag",
		}
		h.workflowArtifact = &workflowArtifact{
			WorkflowID: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		}

		input := h.buildPrivateRegistryInput()

		assert.Equal(t, "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890", input.WorkflowID)
		assert.Equal(t, privateregistryclient.WorkflowStatusActive, input.Status)
		assert.Equal(t, "my-workflow", input.WorkflowName)
		assert.Equal(t, "https://storage.example.com/binary.wasm", input.BinaryURL)
		assert.Equal(t, "zone-a", input.DonFamily)
		require.NotNil(t, input.Tag)
		assert.Equal(t, "v1-tag", *input.Tag)
	})

	t.Run("includes config URL when present", func(t *testing.T) {
		t.Parallel()
		simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)
		defer simulatedEnvironment.Close()

		ctx, buf := simulatedEnvironment.NewRuntimeContextWithBufferedOutput()
		h := newHandler(ctx, buf)
		configURL := "https://storage.example.com/config.yaml"
		h.inputs = Inputs{
			WorkflowName: "my-workflow",
			BinaryURL:    "https://storage.example.com/binary.wasm",
			DonFamily:    "zone-a",
			ConfigURL:    &configURL,
		}
		h.workflowArtifact = &workflowArtifact{
			WorkflowID: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		}

		input := h.buildPrivateRegistryInput()

		require.NotNil(t, input.ConfigURL)
		assert.Equal(t, "https://storage.example.com/config.yaml", *input.ConfigURL)
	})

	t.Run("omits config URL when absent", func(t *testing.T) {
		t.Parallel()
		simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)
		defer simulatedEnvironment.Close()

		ctx, buf := simulatedEnvironment.NewRuntimeContextWithBufferedOutput()
		h := newHandler(ctx, buf)
		h.inputs = Inputs{
			WorkflowName: "my-workflow",
			BinaryURL:    "https://storage.example.com/binary.wasm",
			DonFamily:    "zone-a",
		}
		h.workflowArtifact = &workflowArtifact{
			WorkflowID: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		}

		input := h.buildPrivateRegistryInput()

		assert.Nil(t, input.ConfigURL)
	})

	t.Run("omits tag when empty", func(t *testing.T) {
		t.Parallel()
		simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)
		defer simulatedEnvironment.Close()

		ctx, buf := simulatedEnvironment.NewRuntimeContextWithBufferedOutput()
		h := newHandler(ctx, buf)
		h.inputs = Inputs{
			WorkflowName: "my-workflow",
			BinaryURL:    "https://storage.example.com/binary.wasm",
			DonFamily:    "zone-a",
			WorkflowTag:  "",
		}
		h.workflowArtifact = &workflowArtifact{
			WorkflowID: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		}

		input := h.buildPrivateRegistryInput()

		assert.Nil(t, input.Tag)
	})
}

func TestResolveInputs_PreviewPrivateRegistryFlag(t *testing.T) {
	t.Parallel()

	t.Run("defaults to false", func(t *testing.T) {
		t.Parallel()
		simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)
		defer simulatedEnvironment.Close()

		ctx, buf := simulatedEnvironment.NewRuntimeContextWithBufferedOutput()
		h := newHandler(ctx, buf)
		ctx.Settings = createTestSettings(
			chainsim.TestAddress,
			"eoa",
			"test_workflow",
			"testdata/basic_workflow/main.go",
			"",
		)
		h.settings = ctx.Settings

		inputs, err := h.ResolveInputs(ctx.Viper)
		require.NoError(t, err)
		assert.False(t, inputs.PreviewPrivateRegistry)
	})

	t.Run("set to true via viper", func(t *testing.T) {
		t.Parallel()
		simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)
		defer simulatedEnvironment.Close()

		ctx, buf := simulatedEnvironment.NewRuntimeContextWithBufferedOutput()
		h := newHandler(ctx, buf)
		ctx.Settings = createTestSettings(
			chainsim.TestAddress,
			"eoa",
			"test_workflow",
			"testdata/basic_workflow/main.go",
			"",
		)
		h.settings = ctx.Settings

		ctx.Viper.Set("preview-private-registry", true)

		inputs, err := h.ResolveInputs(ctx.Viper)
		require.NoError(t, err)
		assert.True(t, inputs.PreviewPrivateRegistry)
	})
}
