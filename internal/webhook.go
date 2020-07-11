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
	Name   string `json:"name"`
	URL    string `json:"url"`
	Active bool   `json:"active"`
}

var webhookEvents = []string{
	"push",
	"pull_request",
	"commit_comment",
	"issue_comment",
	"issues",
	"project_card",
	"project_column",
	"project",
	"pull_request_review",
	"pull_request_review_comment",
	"repository",
	"milestone",
}

func (g *GithubIntegration) isOrgWebHookInstalled(manager sdk.WebHookManager, customerID string, integrationInstanceID string, login string) bool {
	return manager.Exists(customerID, integrationInstanceID, refType, login, sdk.WebHookScopeOrg)
}

func (g *GithubIntegration) isWebHookInstalledForRepo(manager sdk.WebHookManager, customerID, integrationInstanceID, orgLogin, repoID string) bool {
	if g.isOrgWebHookInstalled(manager, customerID, integrationInstanceID, orgLogin) {
		return true
	}
	return manager.Exists(customerID, integrationInstanceID, refType, repoID, sdk.WebHookScopeRepo)
}

func (g *GithubIntegration) installRepoWebhookIfRequired(manager sdk.WebHookManager, logger sdk.Logger, client sdk.HTTPClient, customerID string, integrationInstanceID string, login string, repoName string, repoID string) (bool, error) {
	if g.isWebHookInstalledForRepo(manager, customerID, integrationInstanceID, login, repoID) {
		sdk.LogDebug(logger, "webhook is already enabled for this repo", "repo_id", repoID, "name", repoName, "org", login)
		return true, nil
	}
	url, err := g.manager.WebHookManager().Create(customerID, integrationInstanceID, refType, repoID, sdk.WebHookScopeRepo)
	if err != nil {
		if err.Error() == "webhook: disabled" {
			sdk.LogInfo(logger, "webhooks are disabled in dev mode")
			return false, nil // this is ok, just in dev mode disabled
		}
		return false, fmt.Errorf("error creating webhook url for %s: %w", login, err)
	}
	// need to try and install
	params := map[string]interface{}{
		"name": "Pinpoint/" + integrationInstanceID,
		"config": map[string]interface{}{
			"url":          url + "&scope=repo",
			"content_type": "json",
			"insecure_ssl": "0",
			"secret":       integrationInstanceID,
		},
		"events": webhookEvents,
		"active": true,
	}
	kv := make(map[string]interface{})
	resp, err := client.Post(strings.NewReader(sdk.Stringify(params)), &kv, sdk.WithEndpoint("/repos/"+repoName+"/hooks"))
	if err != nil {
		if ok, status, _ := sdk.IsHTTPError(err); ok {
			if status == http.StatusNotFound {
				manager.Errored(customerID, integrationInstanceID, refType, repoID, sdk.WebHookScopeRepo, fmt.Errorf("unauthorized trying to create webhook for this repo"))
				return false, nil
			}
		}
		manager.Errored(customerID, integrationInstanceID, refType, repoID, sdk.WebHookScopeRepo, err)
		return false, nil
	}
	if resp.StatusCode != http.StatusCreated {
		manager.Errored(customerID, integrationInstanceID, refType, repoID, sdk.WebHookScopeRepo, fmt.Errorf("unexpected status code (%d) trying to create webhook", resp.StatusCode))
	}
	return false, nil
}

func (g *GithubIntegration) uninstallRepoWebhook(manager sdk.WebHookManager, client sdk.HTTPClient, customerID string, integrationInstanceID string, orgLogin string, repoName string, repoID string) {
	webhooks := make([]webhookResponse, 0)
	var found bool
	client.Get(&webhooks, sdk.WithEndpoint(fmt.Sprintf("/repos/"+repoName+"/hooks")))
	for _, hook := range webhooks {
		if hook.Name == "Pinpoint/"+integrationInstanceID {
			var res interface{}
			client.Delete(&res, sdk.WithEndpoint(fmt.Sprintf("/repos/"+repoName+"/hooks/%d", hook.ID)))
			found = true
		}
	}
	manager.Delete(customerID, integrationInstanceID, refType, repoID, sdk.WebHookScopeRepo)
	if !found {
		manager.Delete(customerID, integrationInstanceID, refType, orgLogin, sdk.WebHookScopeOrg)
	}
}

func (g *GithubIntegration) unregisterOrgWebhook(manager sdk.WebHookManager, client sdk.HTTPClient, customerID string, integrationInstanceID string, login string) error {
	webhooks := make([]webhookResponse, 0)
	client.Get(&webhooks, sdk.WithEndpoint(fmt.Sprintf("/orgs/"+login+"/hooks")))
	for _, hook := range webhooks {
		if hook.Name == "Pinpoint/"+integrationInstanceID {
			var res interface{}
			client.Delete(&res, sdk.WithEndpoint(fmt.Sprintf("/orgs/"+login+"/hooks/%d", hook.ID)))
		}
	}
	return manager.Delete(customerID, integrationInstanceID, refType, login, sdk.WebHookScopeOrg)
}

func (g *GithubIntegration) registerOrgWebhook(manager sdk.WebHookManager, client sdk.HTTPClient, customerID string, integrationInstanceID string, login string) error {
	if g.isOrgWebHookInstalled(manager, customerID, integrationInstanceID, login) {
		return nil
	}
	webhooks := make([]webhookResponse, 0)
	resp, err := client.Get(&webhooks, sdk.WithEndpoint("/orgs/"+login+"/hooks"))
	if err != nil {
		if resp.StatusCode != http.StatusNotFound {
			manager.Errored(customerID, integrationInstanceID, refType, login, sdk.WebHookScopeOrg, err)
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
		url, err := g.manager.WebHookManager().Create(customerID, integrationInstanceID, refType, login, sdk.WebHookScopeOrg)
		if err != nil {
			manager.Errored(customerID, integrationInstanceID, refType, login, sdk.WebHookScopeOrg, err)
			return fmt.Errorf("error creating webhook url for %s: %w", login, err)
		}
		params := map[string]interface{}{
			"name": "Pinpoint/" + integrationInstanceID,
			"config": map[string]interface{}{
				"url":          url + "&scope=org",
				"content_type": "json",
				"insecure_ssl": "0",
				"secret":       integrationInstanceID,
			},
			"events": webhookEvents,
			"active": true,
		}
		kv := make(map[string]interface{})
		resp, err = client.Post(strings.NewReader(sdk.Stringify(params)), &kv, sdk.WithEndpoint("/orgs/"+login+"/hooks"))
		if err != nil {
			manager.Errored(customerID, integrationInstanceID, refType, login, sdk.WebHookScopeOrg, err)
			return fmt.Errorf("error creating webhook for %s: %w", login, err)
		}
		if resp.StatusCode != http.StatusCreated {
			err := fmt.Errorf("error creating webhook, expected status code 201 but received %d", resp.StatusCode)
			manager.Errored(customerID, integrationInstanceID, refType, login, sdk.WebHookScopeOrg, err)
			return err
		}
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
	var objects []sdk.Model
	switch v := obj.(type) {
	case *github.PushEvent:
		repoLogin := getPushRepoOwnerLogin(v.Repo)
		userManager := NewUserManager(webhook.CustomerID(), []string{repoLogin}, webhook, webhook.State(), webhook.Pipe(), g, webhook.IntegrationInstanceID(), false)
		commits, err := g.fromPushEvent(g.logger, client, userManager, webhook, webhook.CustomerID(), v)
		if err != nil {
			return err
		}
		for _, commit := range commits {
			objects = append(objects, commit)
		}
	case *github.PullRequestEvent:
		repoLogin := getRepoOwnerLogin(v.Repo)
		userManager := NewUserManager(webhook.CustomerID(), []string{repoLogin}, webhook, webhook.State(), webhook.Pipe(), g, webhook.IntegrationInstanceID(), false)
		obj, err := g.fromPullRequestEvent(g.logger, client, userManager, webhook, webhook.CustomerID(), v)
		if err != nil {
			return err
		}
		if obj != nil {
			objects = []sdk.Model{obj}
		}
	case *github.PullRequestReviewEvent:
		repoLogin := getRepoOwnerLogin(v.Repo)
		userManager := NewUserManager(webhook.CustomerID(), []string{repoLogin}, webhook, webhook.State(), webhook.Pipe(), g, webhook.IntegrationInstanceID(), false)
		obj, err := g.fromPullRequestReviewEvent(g.logger, client, userManager, webhook, webhook.CustomerID(), v)
		if err != nil {
			return err
		}
		if obj != nil {
			objects = []sdk.Model{obj}
		}
	case *github.IssueCommentEvent:
		repoLogin := getRepoOwnerLogin(v.Repo)
		if isIssueCommentPR(v) {
			userManager := NewUserManager(webhook.CustomerID(), []string{repoLogin}, webhook, webhook.State(), webhook.Pipe(), g, webhook.IntegrationInstanceID(), false)
			obj, err := g.fromPullRequestCommentEvent(g.logger, client, userManager, webhook, webhook.CustomerID(), v)
			if err != nil {
				return err
			}
			if obj != nil {
				objects = []sdk.Model{obj}
			}
		} else {
			userManager := NewUserManager(webhook.CustomerID(), []string{repoLogin}, webhook, webhook.State(), webhook.Pipe(), g, webhook.IntegrationInstanceID(), false)
			obj, err := g.fromIssueCommentEvent(g.logger, client, userManager, webhook, webhook.CustomerID(), webhook.IntegrationInstanceID(), v)
			if err != nil {
				return err
			}
			if obj != nil {
				objects = []sdk.Model{obj}
			}
		}
	case *github.RepositoryEvent:
		repo, project, capability := g.fromRepositoryEvent(g.logger, webhook.State(), webhook.IntegrationInstanceID(), webhook.CustomerID(), v)
		if err != nil {
			return err
		}
		if repo != nil {
			objects = []sdk.Model{repo}
			if project != nil {
				objects = append(objects, project)
			}
			if capability != nil {
				objects = append(objects, capability)
			}
		}
	case *github.IssuesEvent:
		repoLogin := getRepoOwnerLogin(v.Repo)
		userManager := NewUserManager(webhook.CustomerID(), []string{repoLogin}, webhook, webhook.State(), webhook.Pipe(), g, webhook.IntegrationInstanceID(), false)
		issue, err := g.fromIssueEvent(g.logger, userManager, webhook.IntegrationInstanceID(), webhook.CustomerID(), v)
		if err != nil {
			return err
		}
		if issue != nil {
			objects = []sdk.Model{issue}
		}
	case *github.ProjectEvent:
		repoLogin := getRepoOwnerLogin(v.Repo)
		return g.fetchRepoProject(g.logger, client, webhook.Pipe(), webhook, webhook.CustomerID(), webhook.IntegrationInstanceID(), repoLogin, v.Repo.GetName(), v.Repo.GetNodeID(), v.Project.GetNumber())
	case *github.ProjectCardEvent:
		repoLogin := getRepoOwnerLogin(v.Repo)
		return g.fetchRepoProject(g.logger, client, webhook.Pipe(), webhook, webhook.CustomerID(), webhook.IntegrationInstanceID(), repoLogin, v.Repo.GetName(), v.Repo.GetNodeID(), int(v.GetProjectCard().GetProjectID()))
	case *github.ProjectColumnEvent:
		repoLogin := getRepoOwnerLogin(v.Repo)
		num := getProjectIDfromURL(v.ProjectColumn.GetProjectURL())
		return g.fetchRepoProject(g.logger, client, webhook.Pipe(), webhook, webhook.CustomerID(), webhook.IntegrationInstanceID(), repoLogin, v.Repo.GetName(), v.Repo.GetNodeID(), num)
	case *github.MilestoneEvent:
		repoLogin := getRepoOwnerLogin(v.Repo)
		userManager := NewUserManager(webhook.CustomerID(), []string{repoLogin}, webhook, webhook.State(), webhook.Pipe(), g, webhook.IntegrationInstanceID(), false)
		issue, err := g.fromMilestoneEvent(g.logger, client, userManager, webhook, webhook.CustomerID(), webhook.IntegrationInstanceID(), v)
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
