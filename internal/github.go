package internal

import (
	"sync"

	"github.com/pinpt/agent.next/sdk"
)

// GithubIntegration is an integration for GitHub
type GithubIntegration struct {
	logger  sdk.Logger
	config  sdk.Config
	manager sdk.Manager
	client  sdk.GraphQLClient
	lock    sync.Mutex
}

var _ sdk.Integration = (*GithubIntegration)(nil)

// Start is called when the integration is starting up
func (g *GithubIntegration) Start(logger sdk.Logger, config sdk.Config, manager sdk.Manager) error {
	g.logger = logger
	g.config = config
	g.manager = manager
	sdk.LogInfo(g.logger, "starting")
	return nil
}

// WebHook is called when a webhook is received on behalf of the integration
func (g *GithubIntegration) WebHook(webhook sdk.WebHook) error {
	return nil
}

// Stop is called when the integration is shutting down for cleanup
func (g *GithubIntegration) Stop() error {
	sdk.LogInfo(g.logger, "stopping")
	return nil
}
