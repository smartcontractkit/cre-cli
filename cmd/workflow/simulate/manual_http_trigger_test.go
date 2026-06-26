package simulate

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	"github.com/smartcontractkit/chainlink-common/pkg/capabilities"
	httptypedapi "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/triggers/http"
	"github.com/smartcontractkit/chainlink-common/pkg/config"
	"github.com/smartcontractkit/chainlink-common/pkg/logger"
)

func TestManualHTTPTriggerEnforcesRateLimit(t *testing.T) {
	t.Parallel()

	rateLimit := &config.Rate{
		Limit: rate.Every(time.Hour),
		Burst: 1,
	}
	svc := NewManualHTTPTriggerService(logger.Test(t), defaultHTTPTriggerServerPort, rateLimit)

	_, capErr := svc.RegisterTrigger(
		context.Background(),
		"trigger-1",
		capabilities.RequestMetadata{WorkflowID: "wf-1"},
		&httptypedapi.Config{},
	)
	require.Nil(t, capErr)

	err := svc.ManualTrigger(context.Background(), "trigger-1", &httptypedapi.Payload{Input: []byte(`{"k":"v"}`)})
	require.NoError(t, err)

	err = svc.ManualTrigger(context.Background(), "trigger-1", &httptypedapi.Payload{Input: []byte(`{"k":"v2"}`)})
	require.Error(t, err)
	assert.ErrorIs(t, err, errHTTPTriggerRateLimited)
}
