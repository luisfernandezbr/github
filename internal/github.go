package internal

import (
	"fmt"
	"sync"

	"github.com/pinpt/agent.next/sdk"
)

// GithubIntegration is an integration for GitHub
// easyjson:skip
type GithubIntegration struct {
	logger  sdk.Logger
	config  sdk.Config
	manager sdk.Manager
	lock    sync.Mutex

	testClient sdk.GraphQLClient // only set in testing
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
	// attempt to add an org level web hook
	config := instance.Config()
	state := instance.State()
	if config.IntegrationType == sdk.CloudIntegration && config.OAuth2Auth != nil {
		_, client, err := g.newHTTPClient(g.logger, config)
		if err != nil {
			return fmt.Errorf("error creating http client: %w", err)
		}
		for login, acct := range *config.Accounts {
			if acct.Type == sdk.ConfigAccountTypeOrg {
				if err := g.registerWebhook(instance.CustomerID(), state, client, login, instance.IntegrationInstanceID(), "/orgs/"+login+"/hooks"); err != nil {
					return fmt.Errorf("error creating webhook. %w", err)
				}
			}
		}
	}
	return nil
}

// Dismiss is called when an existing integration instance is removed
func (g *GithubIntegration) Dismiss(instance sdk.Instance) error {
	config := instance.Config()
	if config.IntegrationType == sdk.CloudIntegration && config.OAuth2Auth != nil {
		_, client, err := g.newHTTPClient(g.logger, config)
		if err != nil {
			return fmt.Errorf("error creating http client: %w", err)
		}
		for login, acct := range *config.Accounts {
			if acct.Type == sdk.ConfigAccountTypeOrg {
				if err := g.unregisterWebhook(instance.State(), client, login, instance.IntegrationInstanceID(), "/orgs/"+login+"/hooks"); err != nil {
					sdk.LogError(g.logger, "error unregistering webhook", "login", login, "err", err)
				}
			}
		}
	}
	instance.State().Delete(previousReposStateKey)
	return nil
}

// Stop is called when the integration is shutting down for cleanup
func (g *GithubIntegration) Stop() error {
	sdk.LogInfo(g.logger, "stopping")
	return nil
}
