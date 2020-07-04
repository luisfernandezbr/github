package internal

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/google/go-github/v32/github"
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

func (g *GithubIntegration) isWebHookInstalledForRepo(state sdk.State, customerID, integrationInstanceID, orgLogin, repoName string) bool {
	if g.isOrgWebHookInstalled(state, orgLogin) {
		return true
	}
	key := repoWebhookInstalledStateKeyPrefix + integrationInstanceID + "_" + repoName
	var id int64
	found, _ := state.Get(key, &id)
	return found
}

func (g *GithubIntegration) installRepoWebhookIfRequired(customerID string, logger sdk.Logger, state sdk.State, client sdk.HTTPClient, login string, integrationInstanceID string, repo string) (bool, error) {
	if g.isWebHookInstalledForRepo(state, customerID, integrationInstanceID, login, repo) {
		sdk.LogDebug(logger, "webhook is already enabled for this repo", "repo", repo, "org", login)
		return true, nil
	}
	url, err := g.manager.CreateWebHook(customerID, refType, integrationInstanceID, login)
	if err != nil {
		if err.Error() == "webhook: disabled" {
			sdk.LogInfo(logger, "webhooks are disabled in dev mode")
			return false, nil // this is ok, just in dev mode disabled
		}
		return false, fmt.Errorf("error creating webhook url for %s: %w", login, err)
	}
	// need to try and install
	params := map[string]interface{}{
		"name": "web",
		"config": map[string]interface{}{
			"url":          url,
			"content_type": "json",
			"insecure_ssl": "0",
			"secret":       integrationInstanceID,
		},
		"events": webhookEvents,
		"active": true,
	}
	kv := make(map[string]interface{})
	resp, err := client.Post(strings.NewReader(sdk.Stringify(params)), &kv, sdk.WithEndpoint("/repos/"+repo+"/hooks"))
	if err != nil {
		if err.Error() == "HTTP Error: 404" {
			sdk.LogInfo(logger, "not authorized to create webhooks for repo", "login", login, "repo", repo)
			return false, nil
		}
		return false, fmt.Errorf("error creating webhook for %s: %w", login, err)
	}
	if resp.StatusCode == http.StatusCreated {
		key := repoWebhookInstalledStateKeyPrefix + integrationInstanceID + "_" + repo
		if err := state.Set(key, kv["id"]); err != nil {
			return false, fmt.Errorf("error saving repo %s webhook url to state: %w", repo, err)
		}
		return false, nil // return false just indicating it wasn't already installed
	}
	return false, fmt.Errorf("error saving repo %s webhook url, expected 201 status code but received %v", repo, resp.StatusCode)
}

func (g *GithubIntegration) uninstallRepoWebhook(state sdk.State, client sdk.HTTPClient, login string, integrationInstanceID string, repo string) {
	key := repoWebhookInstalledStateKeyPrefix + integrationInstanceID + "_" + repo
	var id int64
	state.Get(key, &id)
	if id > 0 {
		// delete from github
		kv := make(map[string]interface{})
		client.Delete(&kv, sdk.WithEndpoint(fmt.Sprintf("/repos/"+repo+"/hooks/%d", id)))
	}
	state.Delete(key)
}

func (g *GithubIntegration) unregisterWebhook(state sdk.State, client sdk.HTTPClient, login string, integrationInstanceID string, hookendpoint string) error {
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
			g.uninstallRepoWebhook(state, client, login, integrationInstanceID, repo)
		}
	}
	return nil
}

func (g *GithubIntegration) registerWebhook(customerID string, state sdk.State, client sdk.HTTPClient, login string, integrationInstanceID string, hookendpoint string) error {
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
		if strings.Contains(hook.URL, "event-api") && strings.Contains(hook.URL, "pinpoint.com") && strings.Contains(hook.URL, integrationInstanceID) {
			found = true
			break
		}
	}
	if !found {
		url, err := g.manager.CreateWebHook(customerID, refType, integrationInstanceID, login)
		if err != nil {
			return fmt.Errorf("error creating webhook url for %s: %w", login, err)
		}
		url += "?integration_id=" + integrationInstanceID
		// need to try and install
		params := map[string]interface{}{
			"name": "web",
			"config": map[string]interface{}{
				"url":          url,
				"content_type": "json",
				"insecure_ssl": "0",
				"secret":       integrationInstanceID,
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
	event := webhook.Headers()["x-github-event"]
	sdk.LogInfo(g.logger, "webhook received", "headers", webhook.Headers(), "event", event)
	obj, err := github.ParseWebHook(event, webhook.Bytes())
	if err != nil {
		return err
	}
	client := g.testClient
	if client == nil {
		_, cl, err := g.newGraphClient(g.logger, webhook.Config())
		if err != nil {
			return err
		}
		client = cl
	}
	// TODO: we should probably hash users in state so we don't emit each time we do something
	var objects []sdk.Model
	switch v := obj.(type) {
	case *github.PushEvent:
		userManager := NewUserManager(webhook.CustomerID(), []string{*v.Repo.Owner.Login}, webhook, webhook.Pipe(), g, webhook.IntegrationInstanceID())
		commits, err := g.fromPushEvent(g.logger, client, userManager, webhook, webhook.CustomerID(), v)
		if err != nil {
			return err
		}
		for _, commit := range commits {
			objects = append(objects, commit)
		}
	case *github.PullRequestEvent:
		userManager := NewUserManager(webhook.CustomerID(), []string{*v.Repo.Owner.Login}, webhook, webhook.Pipe(), g, webhook.IntegrationInstanceID())
		obj, err := g.fromPullRequestEvent(g.logger, client, userManager, webhook, webhook.CustomerID(), v)
		if err != nil {
			return err
		}
		if obj != nil {
			objects = []sdk.Model{obj}
		}
	case *github.PullRequestReviewEvent:
		userManager := NewUserManager(webhook.CustomerID(), []string{*v.Repo.Owner.Login}, webhook, webhook.Pipe(), g, webhook.IntegrationInstanceID())
		obj, err := g.fromPullRequestReviewEvent(g.logger, client, userManager, webhook, webhook.CustomerID(), v)
		if err != nil {
			return err
		}
		if obj != nil {
			objects = []sdk.Model{obj}
		}
	case *github.IssueCommentEvent:
		if isIssueCommentPR(v) {
			userManager := NewUserManager(webhook.CustomerID(), []string{*v.Repo.Owner.Login}, webhook, webhook.Pipe(), g, webhook.IntegrationInstanceID())
			obj, err := g.fromPullRequestCommentEvent(g.logger, client, userManager, webhook, webhook.CustomerID(), v)
			if err != nil {
				return err
			}
			if obj != nil {
				objects = []sdk.Model{obj}
			}
		} else {
			userManager := NewUserManager(webhook.CustomerID(), []string{*v.Repo.Owner.Login}, webhook, webhook.Pipe(), g, webhook.IntegrationInstanceID())
			obj, err := g.fromIssueCommentEvent(g.logger, client, userManager, webhook, webhook.CustomerID(), webhook.IntegrationInstanceID(), v)
			if err != nil {
				return err
			}
			if obj != nil {
				objects = []sdk.Model{obj}
			}
		}
	case *github.RepositoryEvent:
		repo, project := g.fromRepositoryEvent(g.logger, webhook.IntegrationInstanceID(), webhook.CustomerID(), v)
		if err != nil {
			return err
		}
		if repo != nil {
			objects = []sdk.Model{repo}
			if project != nil {
				objects = append(objects, project)
			}
		}
	case *github.IssuesEvent:
		userManager := NewUserManager(webhook.CustomerID(), []string{*v.Repo.Owner.Login}, webhook, webhook.Pipe(), g, webhook.IntegrationInstanceID())
		issue, err := g.fromIssueEvent(g.logger, userManager, webhook.IntegrationInstanceID(), webhook.CustomerID(), v)
		if err != nil {
			return err
		}
		if issue != nil {
			objects = []sdk.Model{issue}
		}
	}
	for _, object := range objects {
		sdk.LogDebug(g.logger, "sending webhook to pipe", "data", object.Stringify())
		if err := webhook.Pipe().Write(object); err != nil {
			return err
		}
	}
	return nil
}
