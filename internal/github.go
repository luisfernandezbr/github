package internal

import (
	"fmt"
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

// Start is called when the integration is starting up
func (g *GithubIntegration) Start(logger log.Logger, config sdk.Config, manager sdk.Manager) error {
	if _, ok := config["apikey"]; !ok {
		return fmt.Errorf("missing required apikey")
	}
	g.logger = logger
	g.config = config
	g.manager = manager
	url := config["url"]
	if url == "" {
		url = "https://api.github.com/graphql"
	}
	g.client = manager.GraphQLManager().New(url, map[string]string{
		"Authorization": "bearer " + g.config["apikey"],
	})
	log.Debug(logger, "starting", "url", url)
	return nil
}

// WebHook is called when a webhook is received on behalf of the integration
func (g *GithubIntegration) WebHook(webhook sdk.WebHook) error {
	return nil
}

// Stop is called when the integration is shutting down for cleanup
func (g *GithubIntegration) Stop() error {
	log.Debug(g.logger, "stopping")
	return nil
}
