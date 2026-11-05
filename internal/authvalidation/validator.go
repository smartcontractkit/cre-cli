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
	onRetry        RetryNotifier
}

// RetryNotifier lets callers show progress, since retries can stretch validation to most of a minute.
type RetryNotifier func(nextAttempt, maxAttempts uint, err error)

const (
	defaultMaxAttempts    = 3
	defaultRetryBaseDelay = time.Second
	// Sized so all attempts plus backoff fit in the 1-minute validation timeout.
	defaultAttemptTimeout = 15 * time.Second
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

func (v *Validator) SetRetryNotifier(fn RetryNotifier) {
	v.onRetry = fn
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

	// Refreshing inside the retry loop would apply the per-attempt timeout and could replay a rotated refresh token.
	if err := v.gqlClient.RefreshTokenIfNeeded(ctx); err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
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
		retry.DelayType(retryDelay),
		retry.RetryIf(func(err error) bool {
			if errors.Is(err, graphqlclient.ErrTokenRefresh) || retryAfterExceedsDeadline(ctx, err) {
				return false
			}
			return graphqlclient.IsRetryable(err) || isAttemptTimeout(ctx, err)
		}),
		retry.LastErrorOnly(true),
		retry.Context(ctx),
		retry.OnRetry(func(n uint, err error) {
			lastErr = err
			v.log.Debug().Err(err).Uint("attempt", n+1).Msg("organization info lookup failed")
			// retry-go also calls OnRetry after the final attempt, when nothing will follow.
			if v.onRetry != nil && n+1 < v.maxAttempts {
				v.onRetry(n+2, v.maxAttempts, err)
			}
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

func retryDelay(n uint, err error, config *retry.Config) time.Duration {
	if d := retryAfter(err); d > 0 {
		return d
	}
	return retry.CombineDelay(retry.BackOffDelay, retry.RandomDelay)(n, err, config)
}

// Failing now beats sleeping into the deadline only to fail anyway.
func retryAfterExceedsDeadline(ctx context.Context, err error) bool {
	deadline, ok := ctx.Deadline()
	return ok && retryAfter(err) > time.Until(deadline)
}

func retryAfter(err error) time.Duration {
	var statusErr *graphqlclient.HTTPStatusError
	if errors.As(err, &statusErr) {
		return statusErr.RetryAfter
	}
	return 0
}

// DescribeRetryCause gives a short user-facing reason; full errors are too long for a spinner line.
func DescribeRetryCause(err error) string {
	var statusErr *graphqlclient.HTTPStatusError
	switch {
	case errors.As(err, &statusErr):
		return fmt.Sprintf("returned HTTP %d", statusErr.StatusCode)
	case errors.Is(err, context.DeadlineExceeded):
		return "timed out"
	default:
		return "was unreachable"
	}
}

func orgInfoUnavailableError(reason string) error {
	return fmt.Errorf("authentication failed: %s. Your account may not be fully set up yet — please try again in a few minutes", reason)
}
