package graphqlclient

import (
	"context"
	"fmt"
	"net/http"

	"github.com/machinebox/graphql"

	"github.com/smartcontractkit/cre-cli/internal/auth"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
)

type Client struct {
	client    *graphql.Client
	creds     *credentials.Credentials
	transport *headerTransport
}

func New(creds *credentials.Credentials, environmentSet *environments.EnvironmentSet) *Client {
	ht := newHeaderTransport(creds, auth.NewOAuthService(environmentSet), http.DefaultTransport)
	httpClient := &http.Client{Transport: ht}
	gqlClient := graphql.NewClient(environmentSet.GraphQLURL, graphql.WithHTTPClient(httpClient))

	return &Client{
		client:    gqlClient,
		creds:     creds,
		transport: ht,
	}
}

func (c *Client) SetHeader(key, value string) {
	c.transport.extraHeaders.Set(key, value)
}

func (c *Client) Execute(ctx context.Context, req *graphql.Request, resp any) error {
	if c.creds == nil {
		return fmt.Errorf("credentials not provided")
	}
	return c.client.Run(ctx, req, resp)
}
