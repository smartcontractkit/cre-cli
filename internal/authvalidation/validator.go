package authvalidation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/machinebox/graphql"
	"github.com/rs/zerolog"

	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
)

const queryCreOrganizationInfo = `
query GetCreOrganizationInfo {
	getCreOrganizationInfo {
		orgId
		derivedWorkflowOwners
	}
}`

// ValidationResult holds the data returned by credential validation.
type ValidationResult struct {
	OrgID                string
	DerivedWorkflowOwner string
}

// Validator validates authentication credentials
type Validator struct {
	gqlClient      *graphqlclient.Client
	log            *zerolog.Logger
	timeout        time.Duration
	maxAttempts    uint
	retryBaseDelay time.Duration
	attemptTimeout time.Duration
}

const (
	defaultMaxAttempts    = 3
	defaultRetryBaseDelay = time.Second
	// Keeps one hung request from consuming the whole validation timeout and starving retries.
	defaultAttemptTimeout = 20 * time.Second
)

// NewValidator creates a new credential validator
func NewValidator(creds *credentials.Credentials, environmentSet *environments.EnvironmentSet, log *zerolog.Logger) *Validator {
	gqlClient := graphqlclient.New(creds, environmentSet, log)
	return &Validator{
		gqlClient:      gqlClient,
		log:            log,
		timeout:        time.Minute,
		maxAttempts:    defaultMaxAttempts,
		retryBaseDelay: defaultRetryBaseDelay,
		attemptTimeout: defaultAttemptTimeout,
	}
}

func (v *Validator) SetServiceTimeout(timeout time.Duration) {
	v.timeout = timeout
}

func (v *Validator) CreateServiceContextWithTimeout(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, v.timeout) //nolint:gosec // G118 -- cancel is deferred by callers
}

// ValidateCredentials validates the provided credentials by making a lightweight GraphQL query
// and returns organization info including the derived workflow owner.
// The GraphQL client automatically handles token refresh if needed.
func (v *Validator) ValidateCredentials(validationCtx context.Context, creds *credentials.Credentials) (*ValidationResult, error) {
	ctx, cancel := v.CreateServiceContextWithTimeout(validationCtx)
	defer cancel()

	if creds == nil {
		return nil, fmt.Errorf("credentials not provided")
	}

	if creds.IsValidated {
		return nil, nil
	}

	var lastErr error
	result, err := retry.DoWithData(
		func() (*ValidationResult, error) {
			attemptCtx, cancelAttempt := context.WithTimeout(ctx, v.attemptTimeout)
			defer cancelAttempt()
			return v.fetchOrganizationInfo(attemptCtx)
		},
		retry.Attempts(v.maxAttempts),
		retry.Delay(v.retryBaseDelay),
		retry.MaxJitter(v.retryBaseDelay/2),
		retry.DelayType(retry.CombineDelay(retry.BackOffDelay, retry.RandomDelay)),
		retry.RetryIf(func(err error) bool {
			return graphqlclient.IsRetryable(err) || isAttemptTimeout(ctx, err)
		}),
		retry.LastErrorOnly(true),
		retry.Context(ctx),
		retry.OnRetry(func(n uint, err error) {
			lastErr = err
			v.log.Debug().Err(err).Uint("attempt", n+1).Msg("organization info lookup failed")
		}),
	)
	if err != nil {
		// retry-go returns only the context error on cancellation; keep the API error that caused the retries.
		if lastErr != nil && errors.Is(err, context.Cause(ctx)) {
			return nil, fmt.Errorf("%w (retry aborted: %w)", lastErr, err)
		}
		return nil, err
	}

	creds.OrgID = result.OrgID
	return result, nil
}

func (v *Validator) fetchOrganizationInfo(ctx context.Context) (*ValidationResult, error) {
	var respEnvelope struct {
		GetCreOrganizationInfo *struct {
			OrgID                 string   `json:"orgId"`
			DerivedWorkflowOwners []string `json:"derivedWorkflowOwners"`
		} `json:"getCreOrganizationInfo"`
	}

	if err := v.gqlClient.Execute(ctx, graphql.NewRequest(queryCreOrganizationInfo), &respEnvelope); err != nil {
		return nil, fmt.Errorf("authentication failed: unable to retrieve organization info: %w", err)
	}

	info := respEnvelope.GetCreOrganizationInfo
	if info == nil {
		return nil, orgInfoUnavailableError("the CRE API returned no organization for this account")
	}

	var missing []string
	if info.OrgID == "" {
		missing = append(missing, "orgId")
	}
	if len(info.DerivedWorkflowOwners) == 0 {
		missing = append(missing, "derivedWorkflowOwners")
	}
	if len(missing) > 0 {
		return nil, orgInfoUnavailableError("organization info is missing " + strings.Join(missing, ", "))
	}

	return &ValidationResult{
		OrgID:                info.OrgID,
		DerivedWorkflowOwner: info.DerivedWorkflowOwners[0],
	}, nil
}

// isAttemptTimeout distinguishes a per-attempt deadline (retryable) from the overall validation deadline.
func isAttemptTimeout(ctx context.Context, err error) bool {
	return errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil
}

func orgInfoUnavailableError(reason string) error {
	return fmt.Errorf("authentication failed: %s. Your account may not be fully set up yet — please try again in a few minutes", reason)
}
