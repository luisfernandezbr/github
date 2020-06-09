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
	g.logger = sdk.LogWith(logger, "pkg", "github")
	g.config = config
	g.manager = manager
	sdk.LogInfo(g.logger, "starting")
	return nil
}

// Enroll is called when a new integration instance is added
func (g *GithubIntegration) Enroll(instance sdk.Instance) error {
	// FIXME: add the web hook for this integration
	return nil
}

// Dismiss is called when an existing integration instance is removed
func (g *GithubIntegration) Dismiss(instance sdk.Instance) error {
	// FIXME: remove integration
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
