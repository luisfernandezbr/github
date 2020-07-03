package internal

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/pinpt/agent.next/sdk"
)

type webhookResponse struct {
	ID     int64  `json:"id"`
	URL    string `json:"url"`
	Active bool   `json:"active"`
}

const (
	orgWebhookInstalledStateKeyPrefix  = "org_webhook_"
	repoWebhookInstalledStateKeyPrefix = "repo_webhook_"
)

var webhookEvents = []string{"push", "pull_request", "commit_comment", "issue_comment", "issues", "project_card", "project_column", "project", "pull_request_review", "pull_request_review_comment", "repository", "milestone"}

func (g *GithubIntegration) isOrgWebHookInstalled(state sdk.State, login string) bool {
	key := orgWebhookInstalledStateKeyPrefix + login
	return state.Exists(key)
}

func (g *GithubIntegration) installRepoWebhookIfRequired(customerID string, logger sdk.Logger, state sdk.State, client sdk.HTTPClient, login string, integrationID string, repo string) error {
	if g.isOrgWebHookInstalled(state, login) {
		return nil
	}
	key := repoWebhookInstalledStateKeyPrefix + integrationID + "_" + repo
	var id int64
	found, err := state.Get(key, &id)
	if err != nil {
		return fmt.Errorf("error fetching webhook state key: %w", err)
	}
	if found {
		sdk.LogInfo(logger, "webhook already exists for this repo", "name", repo, "integration_id", integrationID)
		return nil
	}
	url, err := g.manager.CreateWebHook(customerID, refType, integrationID, login)
	if err != nil {
		if err.Error() == "webhook: disabled" {
			sdk.LogInfo(logger, "webhooks are disabled in dev mode")
			return nil // this is ok, just in dev mode disabled
		}
		return fmt.Errorf("error creating webhook url for %s: %w", login, err)
	}
	// need to try and install
	params := map[string]interface{}{
		"name": "web",
		"config": map[string]interface{}{
			"url":          url,
			"content_type": "json",
			"insecure_ssl": "0",
			"secret":       integrationID,
		},
		"events": webhookEvents,
		"active": true,
	}
	kv := make(map[string]interface{})
	resp, err := client.Post(strings.NewReader(sdk.Stringify(params)), &kv, sdk.WithEndpoint("/repos/"+repo+"/hooks"))
	if err != nil {
		if err.Error() == "HTTP Error: 404" {
			sdk.LogInfo(logger, "not authorized to create webhooks for repo", "login", login, "repo", repo)
			return nil
		}
		return fmt.Errorf("error creating webhook for %s: %w", login, err)
	}
	if resp.StatusCode == http.StatusCreated {
		if err := state.Set(key, kv["id"]); err != nil {
			return fmt.Errorf("error saving repo %s webhook url to state: %w", repo, err)
		}
		return nil
	}
	return fmt.Errorf("error saving repo %s webhook url, expected 201 status code but received %v", repo, resp.StatusCode)
}

func (g *GithubIntegration) uninstallRepoWebhook(state sdk.State, client sdk.HTTPClient, login string, integrationID string, repo string) {
	key := repoWebhookInstalledStateKeyPrefix + integrationID + "_" + repo
	var id int64
	state.Get(key, &id)
	if id > 0 {
		// delete from github
		kv := make(map[string]interface{})
		client.Delete(&kv, sdk.WithEndpoint(fmt.Sprintf("/repos/"+repo+"/hooks/%d", id)))
	}
	state.Delete(key)
}

func (g *GithubIntegration) unregisterWebhook(state sdk.State, client sdk.HTTPClient, login string, integrationID string, hookendpoint string) error {
	key := orgWebhookInstalledStateKeyPrefix + login
	if g.isOrgWebHookInstalled(state, login) {
		var id int64
		found, err := state.Get(key, &id)
		if err != nil {
			return fmt.Errorf("error fetching webhook state key: %w", err)
		}
		if found {
			// delete the state
			state.Delete(key)
			kv := make(map[string]interface{})
			// delete the org webhook
			if _, err := client.Delete(&kv, sdk.WithEndpoint(fmt.Sprintf("%s/%d", hookendpoint, id))); err != nil {
				return err
			}
		}
	} else {
		// go through all the repo hooks and remove
		previousRepos := make(map[string]*sdk.SourceCodeRepo)
		if state.Exists(previousReposStateKey) {
			state.Get(previousReposStateKey, &previousRepos)
		}
		for repo := range previousRepos {
			g.uninstallRepoWebhook(state, client, login, integrationID, repo)
		}
	}
	return nil
}

func (g *GithubIntegration) registerWebhook(customerID string, state sdk.State, client sdk.HTTPClient, login string, integrationID string, hookendpoint string) error {
	if g.isOrgWebHookInstalled(state, login) {
		return nil
	}
	webhooks := make([]webhookResponse, 0)
	resp, err := client.Get(&webhooks, sdk.WithEndpoint(hookendpoint))
	if err != nil {
		if resp.StatusCode != http.StatusNotFound {
			return err
		}
	}
	var found bool
	for _, hook := range webhooks {
		if strings.Contains(hook.URL, "event-api") && strings.Contains(hook.URL, "pinpoint.com") && strings.Contains(hook.URL, integrationID) {
			found = true
			break
		}
	}
	if !found {
		url, err := g.manager.CreateWebHook(customerID, refType, integrationID, login)
		if err != nil {
			return fmt.Errorf("error creating webhook url for %s: %w", login, err)
		}
		url += "?integration_id=" + integrationID
		// need to try and install
		params := map[string]interface{}{
			"name": "web",
			"config": map[string]interface{}{
				"url":          url,
				"content_type": "json",
				"insecure_ssl": "0",
				"secret":       integrationID,
			},
			"events": webhookEvents,
			"active": true,
		}
		kv := make(map[string]interface{})
		resp, err = client.Post(strings.NewReader(sdk.Stringify(params)), &kv, sdk.WithEndpoint(hookendpoint))
		if err != nil {
			return fmt.Errorf("error creating webhook for %s: %w", login, err)
		}
		if resp.StatusCode == http.StatusCreated {
			key := orgWebhookInstalledStateKeyPrefix + login
			if err := state.Set(key, kv["id"]); err != nil {
				return fmt.Errorf("error saving webhook url to state: %w", err)
			}
			return nil
		}
		return fmt.Errorf("error saving webhook url, expected 201 status code but received %v", resp.StatusCode)
	}
	return nil
}

// WebHook is called when a webhook is received on behalf of the integration
func (g *GithubIntegration) WebHook(webhook sdk.WebHook) error {
	// FIXME: implement this
	sdk.LogInfo(g.logger, "webhook received", "payload", sdk.Stringify(webhook.Data()))
	return nil
}
