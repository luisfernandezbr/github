package internal

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/google/go-github/v32/github"
	"github.com/pinpt/agent/v4/sdk"
)

type webhookResponse struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
	Config struct {
		URL string `json:"url"`
	} `json:"config"`
	URL string `json:"url"`
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

const hookVersion = "1" // change this to upgrade the hook in case the events change

func (g *GithubIntegration) isOrgWebHookInstalled(manager sdk.WebHookManager, customerID string, integrationInstanceID string, login string) bool {
	if manager.Exists(customerID, integrationInstanceID, refType, login, sdk.WebHookScopeOrg) {
		theurl, _ := manager.HookURL(customerID, integrationInstanceID, refType, login, sdk.WebHookScopeOrg)
		// check and see if we need to upgrade our hook
		if !strings.Contains(theurl, "version="+hookVersion) {
			manager.Delete(customerID, integrationInstanceID, refType, login, sdk.WebHookScopeOrg)
			return false
		}
		return true
	}
	return false
}

// func (g *GithubIntegration) isWebHookInstalledForRepo(manager sdk.WebHookManager, client sdk.HTTPClient, customerID, integrationInstanceID, orgLogin, repoRefID, repoName string) (bool, error) {
// 	if g.isOrgWebHookInstalled(manager, customerID, integrationInstanceID, orgLogin) {
// 		return true, nil
// 	}
// 	if manager.Exists(customerID, integrationInstanceID, refType, repoRefID, sdk.WebHookScopeRepo) {
// 		theurl, _ := manager.HookURL(customerID, integrationInstanceID, refType, repoRefID, sdk.WebHookScopeRepo)
// 		// check and see if we need to upgrade our hook
// 		if !strings.Contains(theurl, "version="+hookVersion) {
// 			manager.Delete(customerID, integrationInstanceID, refType, repoRefID, sdk.WebHookScopeRepo)
// 			// TODO(robin): remove hook from github
// 			return false, nil
// 		}
// 		installed, err := g.isWebhookInstalledForRepoInGithub(client, repoName, theurl)
// 		if err != nil {
// 			return false, err
// 		}
// 		if !installed {
// 			manager.Delete(customerID, integrationInstanceID, refType, repoRefID, sdk.WebHookScopeRepo)
// 		}
// 		return installed, nil
// 	}
// 	return false, nil
// }

// func (g *GithubIntegration) isWebhookInstalledForRepoInGithub(client sdk.HTTPClient, repoName string, url string) (bool, error) {
// 	webhooks, err := getInstalledWebhooks(client, repoName)
// 	if err != nil {
// 		return false, err
// 	}
// 	for _, wh := range webhooks {
// 		if wh.Config.URL == url {
// 			return true, nil
// 		}
// 	}
// 	return false, nil
// }

func getInstalledWebhooks(client sdk.HTTPClient, repoName string) ([]webhookResponse, error) {
	var webhooks []webhookResponse
	if resp, err := client.Get(&webhooks, sdk.WithEndpoint("/repos/"+repoName+"/hooks")); err != nil {
		if resp.StatusCode == 404 {
			return nil, nil
		}
		return nil, fmt.Errorf("error getting a repos webhooks from github: %w", err)
	}
	return webhooks, nil
}

func registerWebhook(logger sdk.Logger, client sdk.HTTPClient, repoName string, url string, secret string) error {
	// need to try and install
	params := map[string]interface{}{
		"name": "web",
		"config": map[string]interface{}{
			"url":          url,
			"content_type": "json",
			"insecure_ssl": "0",
			"secret":       secret,
		},
		"events": webhookEvents,
		"active": true,
	}
	kv := make(map[string]interface{})
	sdk.LogDebug(logger, "creating a repo webhook on github", "name", repoName, "params", sdk.Stringify(params))
	resp, err := client.Post(strings.NewReader(sdk.Stringify(params)), &kv, sdk.WithEndpoint("/repos/"+repoName+"/hooks"))
	if err != nil {
		if resp != nil {
			sdk.LogDebug(logger, "error creating a repo webhook on github", "err", err, "repo_id", repoName, "error_body", string(resp.Body))
		}
		return fmt.Errorf("error creating repo webhook: %w", err)
	}
	sdk.LogDebug(logger, "webhook result", "data", sdk.Stringify(kv), "status", resp.StatusCode, "repo", repoName)
	if resp.StatusCode != http.StatusCreated {
		// manager.Errored(customerID, integrationInstanceID, refType, repoRefID, sdk.WebHookScopeRepo, fmt.Errorf("unexpected status code (%d) trying to create webhook", resp.StatusCode))
		return fmt.Errorf("unexpected status code (%d) trying to create webhook", resp.StatusCode)
	}
	return nil
}

func deleteWebhook(client sdk.HTTPClient, repoName string, hookID int) error {
	if repoName == "" {
		return fmt.Errorf("repo name was empty")
	}
	resp, err := client.Delete(nil, sdk.WithEndpoint(fmt.Sprintf("/repos/%s/hooks/%d", repoName, hookID)))
	if err != nil {
		if resp.StatusCode == http.StatusNotFound {
			return nil
		}
		return fmt.Errorf("error removing repo webhook: %w", err)
	}
	return nil
}

func isSharedWebhook(url string) bool {
	return strings.Contains(url, "webhook.api")
}

func (g *GithubIntegration) installRepoWebhookIfRequired(manager sdk.WebHookManager, logger sdk.Logger, client sdk.HTTPClient, customerID string, integrationInstanceID string, login string, repoName string, repoRefID string) (bool, error) {
	// Get webhooks that are installed
	webhooks, err := getInstalledWebhooks(client, repoName)
	if err != nil {
		return false, fmt.Errorf("error getting installed webhooks: %w", err)
	}
	var oldWebhooks []webhookResponse
	for _, webhook := range webhooks {
		if manager.IsPinpointWebhook(webhook.Config.URL) {
			if isSharedWebhook(webhook.Config.URL) {
				// if new webhook exists return true
				return true, nil
			}
			oldWebhooks = append(oldWebhooks, webhook)
		}
	}
	// remove any old webhooks
	for _, oldWebhook := range oldWebhooks {
		sdk.LogDebug(logger, "removing old webhook", "url", oldWebhook.Config.URL, "id", oldWebhook.ID)
		if err := deleteWebhook(client, repoName, oldWebhook.ID); err != nil {
			sdk.LogDebug(logger, "error removing old pinpoint webhook", "err", err)
		}
	}
	url, err := manager.CreateSharedWebhook(customerID, integrationInstanceID, refType, repoRefID, sdk.WebHookScopeRepo)
	if err != nil {
		return false, fmt.Errorf("error creating webhook url for %s: %w", login, err)
	}
	// add new webhook
	if err := registerWebhook(logger, client, repoName, url, manager.Secret()); err != nil {
		// TODO(robin): add back manager errors
		// manager.Errored(customerID, integrationInstanceID, refType, repoRefID, sdk.WebHookScopeRepo, err)
		sdk.LogWarn(logger, "error registering webhook", "err", err)
		return false, nil
	}
	// TODO(robin): sync repowebhooks using manager.Create, manager.Errored
	sdk.LogDebug(logger, "creating a repo webhook", "name", repoName, "repo_id", repoRefID, "org", login)
	return true, nil
}

func (g *GithubIntegration) uninstallRepoWebhook(logger sdk.Logger, manager sdk.WebHookManager, client sdk.HTTPClient, customerID string, integrationInstanceID string, orgLogin string, repoName string, repoRefID string) {
	webhooks, _ := getInstalledWebhooks(client, repoName)
	var found bool
	for _, hook := range webhooks {
		// only uninstall legacy webhooks
		sdk.LogDebug(logger, "inspecting repo webhook", "name", repoName, "url", hook.URL, "hookid", hook.ID, "hookurl", hook.Config.URL, "id", integrationInstanceID)
		if strings.Contains(hook.Config.URL, integrationInstanceID) {
			var res interface{}
			client.Delete(&res, sdk.WithEndpoint(fmt.Sprintf("/repos/"+repoName+"/hooks/%d", hook.ID)))
			found = true
			sdk.LogDebug(logger, "deleted repo webhook", "res", sdk.Stringify(res), "name", repoName, "hookid", hook.ID)
		}
	}
	if !found {
		sdk.LogDebug(logger, "no repo webhook found for repo: "+repoName)
	}
	manager.Delete(customerID, integrationInstanceID, refType, repoRefID, sdk.WebHookScopeRepo)
	if !found && orgLogin != "" {
		manager.Delete(customerID, integrationInstanceID, refType, orgLogin, sdk.WebHookScopeOrg)
	}
}

func (g *GithubIntegration) unregisterOrgWebhook(manager sdk.WebHookManager, client sdk.HTTPClient, customerID string, integrationInstanceID string, login string) error {
	webhooks := make([]webhookResponse, 0)
	client.Get(&webhooks, sdk.WithEndpoint(fmt.Sprintf("/orgs/"+login+"/hooks")))
	for _, hook := range webhooks {
		if strings.Contains(hook.URL, integrationInstanceID) {
			var res interface{}
			client.Delete(&res, sdk.WithEndpoint(fmt.Sprintf("/orgs/"+login+"/hooks/%d", hook.ID)))
		}
	}
	return manager.Delete(customerID, integrationInstanceID, refType, login, sdk.WebHookScopeOrg)
}

func (g *GithubIntegration) registerOrgWebhook(logger sdk.Logger, manager sdk.WebHookManager, client sdk.HTTPClient, customerID string, integrationInstanceID string, login string) error {
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
		if strings.Contains(hook.URL, "event-api") && strings.Contains(hook.URL, "pinpoint.com") && strings.Contains(hook.Config.URL, integrationInstanceID) {
			found = true
			break
		}
	}
	if !found {
		url, err := g.manager.WebHookManager().Create(customerID, integrationInstanceID, refType, login, sdk.WebHookScopeOrg, "scope=org", "version="+hookVersion)
		if err != nil {
			manager.Errored(customerID, integrationInstanceID, refType, login, sdk.WebHookScopeOrg, err)
			return nil
		}
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
		resp, err = client.Post(strings.NewReader(sdk.Stringify(params)), &kv, sdk.WithEndpoint("/orgs/"+login+"/hooks"))
		if err != nil {
			if ok, status, _ := sdk.IsHTTPError(err); ok && status == http.StatusNotFound {
				sdk.LogInfo(logger, "couldn't install an org webhook, unauthorized", "org", login)
				g.manager.WebHookManager().Delete(customerID, integrationInstanceID, refType, login, sdk.WebHookScopeOrg)
				return nil
			}
			manager.Errored(customerID, integrationInstanceID, refType, login, sdk.WebHookScopeOrg, err)
			return nil
		}
		if resp.StatusCode != http.StatusCreated {
			err := fmt.Errorf("error creating webhook, expected status code 201 but received %d", resp.StatusCode)
			manager.Errored(customerID, integrationInstanceID, refType, login, sdk.WebHookScopeOrg, err)
			return nil
		}
	}
	return nil
}

// WebHook is called when a webhook is received on behalf of the integration
func (g *GithubIntegration) WebHook(webhook sdk.WebHook) error {
	logger := webhook.Logger()
	event := webhook.Headers()["x-github-event"]
	sdk.LogInfo(logger, "webhook received", "headers", webhook.Headers(), "event", event)
	obj, err := github.ParseWebHook(event, webhook.Bytes())
	if err != nil {
		return err
	}
	client := g.testClient
	if client == nil {
		_, cl, err := g.newGraphClient(logger, webhook.Config())
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
		commits, err := g.fromPushEvent(logger, userManager, webhook.CustomerID(), v)
		if err != nil {
			return err
		}
		for _, commit := range commits {
			objects = append(objects, commit)
		}
	case *github.PullRequestEvent:
		repoLogin := getRepoOwnerLogin(v.Repo)
		userManager := NewUserManager(webhook.CustomerID(), []string{repoLogin}, webhook, webhook.State(), webhook.Pipe(), g, webhook.IntegrationInstanceID(), false)
		if v.Action != nil && (*v.Action == "review_requested" || *v.Action == "review_request_removed") {
			obj, err := g.fromPullRequestReviewRequestedEvent(logger, client, userManager, webhook, webhook.CustomerID(), v)
			if err != nil {
				return err
			}
			if obj != nil {
				objects = []sdk.Model{obj}
			}
		} else {
			obj, err := g.fromPullRequestEvent(logger, client, userManager, webhook, webhook.CustomerID(), v)
			if err != nil {
				return err
			}
			if obj != nil {
				objects = []sdk.Model{obj}
			}
		}
	case *github.PullRequestReviewEvent:
		repoLogin := getRepoOwnerLogin(v.Repo)
		userManager := NewUserManager(webhook.CustomerID(), []string{repoLogin}, webhook, webhook.State(), webhook.Pipe(), g, webhook.IntegrationInstanceID(), false)
		obj, err := g.fromPullRequestReviewEvent(logger, client, userManager, webhook, webhook.CustomerID(), v)
		if err != nil {
			return err
		}
		if obj != nil {
			objects = []sdk.Model{obj}
		}
	case *github.CommitCommentEvent:
		// NOTE: we don't really have commit comments in the model right now
	case *github.PullRequestReviewCommentEvent:
		// NOTE: we don't really have pull request review comments in the model right now
		break
	case *github.IssueCommentEvent:
		repoLogin := getRepoOwnerLogin(v.Repo)
		if isIssueCommentPR(v) {
			userManager := NewUserManager(webhook.CustomerID(), []string{repoLogin}, webhook, webhook.State(), webhook.Pipe(), g, webhook.IntegrationInstanceID(), false)
			obj, err := g.fromPullRequestCommentEvent(logger, client, userManager, webhook, webhook.CustomerID(), v)
			if err != nil {
				return err
			}
			if obj != nil {
				objects = []sdk.Model{obj}
			}
		} else {
			userManager := NewUserManager(webhook.CustomerID(), []string{repoLogin}, webhook, webhook.State(), webhook.Pipe(), g, webhook.IntegrationInstanceID(), false)
			obj, err := g.fromIssueCommentEvent(logger, client, userManager, webhook, webhook.CustomerID(), webhook.IntegrationInstanceID(), v)
			if err != nil {
				return err
			}
			if obj != nil {
				objects = []sdk.Model{obj}
			}
		}
	case *github.RepositoryEvent:
		repo, project, capability := g.fromRepositoryEvent(logger, webhook.State(), webhook.IntegrationInstanceID(), webhook.CustomerID(), v)
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
		issue, err := g.fromIssueEvent(logger, userManager, webhook.IntegrationInstanceID(), webhook.CustomerID(), v)
		if err != nil {
			return err
		}
		if issue != nil {
			objects = []sdk.Model{issue}
		}
	case *github.ProjectEvent:
		return g.fetchRepoProject(logger, client, webhook.Pipe(), webhook, webhook.CustomerID(), webhook.IntegrationInstanceID(), v.Repo.GetFullName(), v.Repo.GetNodeID(), v.Project.GetNumber())
	case *github.ProjectCardEvent:
		return g.fetchRepoProject(logger, client, webhook.Pipe(), webhook, webhook.CustomerID(), webhook.IntegrationInstanceID(), v.Repo.GetFullName(), v.Repo.GetNodeID(), int(v.GetProjectCard().GetProjectID()))
	case *github.ProjectColumnEvent:
		num, err := getProjectIDfromURL(v.ProjectColumn.GetProjectURL())
		if err != nil {
			return fmt.Errorf("error getting project id: %w", err)
		}
		return g.fetchRepoProject(logger, client, webhook.Pipe(), webhook, webhook.CustomerID(), webhook.IntegrationInstanceID(), v.Repo.GetFullName(), v.Repo.GetNodeID(), num)
	case *github.MilestoneEvent:
		repoLogin := getRepoOwnerLogin(v.Repo)
		userManager := NewUserManager(webhook.CustomerID(), []string{repoLogin}, webhook, webhook.State(), webhook.Pipe(), g, webhook.IntegrationInstanceID(), false)
		issue, err := g.fromMilestoneEvent(logger, client, userManager, webhook, webhook.CustomerID(), webhook.IntegrationInstanceID(), v)
		if err != nil {
			return err
		}
		if issue != nil {
			objects = []sdk.Model{issue}
		}
	}
	for _, object := range objects {
		sdk.LogDebug(logger, "sending webhook to pipe", "data", object.Stringify())
		if err := webhook.Pipe().Write(object); err != nil {
			return err
		}
	}
	return nil
}
