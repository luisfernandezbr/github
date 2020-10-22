package internal

import (
	"fmt"
	"sync"
	"time"

	"github.com/pinpt/agent/v4/sdk"
)

// GithubIntegration is an integration for GitHub
// easyjson:skip
type GithubIntegration struct {
	config  sdk.Config
	manager sdk.Manager
	lock    sync.Mutex

	testClient sdk.GraphQLClient // only set in testing
}

var _ sdk.Integration = (*GithubIntegration)(nil)

// Start is called when the integration is starting up
func (g *GithubIntegration) Start(logger sdk.Logger, config sdk.Config, manager sdk.Manager) error {
	g.config = config
	g.manager = manager
	sdk.LogInfo(logger, "starting")
	return nil
}

// Enroll is called when a new integration instance is added
func (g *GithubIntegration) Enroll(instance sdk.Instance) error {
	// attempt to add an org level web hook
	started := time.Now()
	logger := instance.Logger()
	sdk.LogInfo(logger, "enroll started")
	config := instance.Config()
	if config.IntegrationType == sdk.CloudIntegration && config.OAuth2Auth != nil {
		_, client, err := g.newHTTPClient(logger, config)
		if err != nil {
			return fmt.Errorf("error creating http client: %w", err)
		}
		for login, acct := range *config.Accounts {
			if acct.Selected != nil && !*acct.Selected {
				continue
			}
			if acct.Type == sdk.ConfigAccountTypeOrg {
				if err := g.registerOrgWebhook(logger, g.manager.WebHookManager(), client, instance.CustomerID(), instance.IntegrationInstanceID(), login); err != nil {
					g.manager.WebHookManager().Errored(instance.CustomerID(), instance.IntegrationInstanceID(), refType, login, sdk.WebHookScopeOrg, err)
				}
			}
		}
	}
	sdk.LogInfo(logger, "enroll finished", "duration", time.Since(started))
	return nil
}

// Dismiss is called when an existing integration instance is removed
func (g *GithubIntegration) Dismiss(instance sdk.Instance) error {
	logger := instance.Logger()
	started := time.Now()
	sdk.LogInfo(logger, "dismiss started")
	config := instance.Config()
	_, client, err := g.newHTTPClient(logger, config)
	if err != nil {
		return fmt.Errorf("error creating http client: %w", err)
	}
	if config.IntegrationType == sdk.CloudIntegration && config.OAuth2Auth != nil {
		for login, acct := range *config.Accounts {
			if acct.Type == sdk.ConfigAccountTypeOrg {
				if err := g.unregisterOrgWebhook(g.manager.WebHookManager(), client, instance.CustomerID(), instance.IntegrationInstanceID(), login); err != nil {
					sdk.LogError(logger, "error unregistering webhook", "login", login, "err", err)
				}
			}
		}
	}
	state := instance.State()
	if state.Exists(previousReposStateKey) {
		previousRepos := make(map[string]*sdk.SourceCodeRepo)
		if _, err := state.Get(previousReposStateKey, &previousRepos); err != nil {
			sdk.LogError(logger, "error fetching previous repos state", "err", err)
		}
		for _, repo := range previousRepos {
			sdk.LogDebug(logger, "de-previsioning repo", "name", repo.Name, "id", repo.RefID)
			// remove the webhook for the repo
			g.uninstallRepoWebhook(logger, g.manager.WebHookManager(), client, instance.CustomerID(), instance.IntegrationInstanceID(), "", repo.Name, repo.RefID)
			// deactivate the repo
			repo.Active = false
			repo.UpdatedAt = sdk.EpochNow()
			instance.Pipe().Write(repo)
		}
	}
	if state.Exists(previousProjectsStateKey) {
		previousProjects := make(map[string]*sdk.WorkProject)
		if _, err := state.Get(previousProjectsStateKey, &previousProjects); err != nil {
			sdk.LogError(logger, "error fetching previous projects state", "err", err)
		}
		for _, project := range previousProjects {
			sdk.LogDebug(logger, "de-previsioning project", "name", project.Name, "id", project.RefID)
			// deactivate the project
			project.Active = false
			project.UpdatedAt = sdk.EpochNow()
			instance.Pipe().Write(project)
		}
	}
	// clean up our state keys
	state.Delete(previousReposStateKey)
	state.Delete(previousProjectsStateKey)
	sdk.LogInfo(logger, "dismiss completed", "duration", time.Since(started), "customer_id", instance.CustomerID(), "integration_instance_id", instance.IntegrationInstanceID())
	return nil
}

// Stop is called when the integration is shutting down for cleanup
func (g *GithubIntegration) Stop(logger sdk.Logger) error {
	sdk.LogInfo(logger, "stopping")
	return nil
}
