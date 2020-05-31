package internal

import (
	"sync"

	"github.com/pinpt/agent.next/sdk"
	"github.com/pinpt/go-common/log"
)

// GithubIntegration is an integration for GitHub
type GithubIntegration struct {
	logger  log.Logger
	config  sdk.Config
	manager sdk.Manager
	client  sdk.GraphQLClient
	lock    sync.Mutex
}

var _ sdk.Integration = (*GithubIntegration)(nil)

// RefType should return the integration ref_type (the short, unique identifier of the integration)
func (g *GithubIntegration) RefType() string {
	return refType
}

// Start is called when the integration is starting up
func (g *GithubIntegration) Start(logger log.Logger, config sdk.Config, manager sdk.Manager) error {
	g.logger = logger
	g.config = config
	g.manager = manager
	log.Info(g.logger, "starting")
	return nil
}

// WebHook is called when a webhook is received on behalf of the integration
func (g *GithubIntegration) WebHook(webhook sdk.WebHook) error {
	return nil
}

// Stop is called when the integration is shutting down for cleanup
func (g *GithubIntegration) Stop() error {
	log.Info(g.logger, "stopping")
	return nil
}
