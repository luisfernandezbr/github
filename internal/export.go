package internal

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	easyjson "github.com/mailru/easyjson"
	"github.com/pinpt/agent.next/sdk"
	"github.com/pinpt/go-common/v10/datetime"
)

const (
	defaultPageSize                  = 50
	defaultRetryPageSize             = 25
	defaultPullRequestCommitPageSize = 100
	previousReposStateKey            = "previous_repos"
)

type job func(export sdk.Export, pipe sdk.Pipe) error

func (g *GithubIntegration) checkForRetryableError(logger sdk.Logger, control sdk.Control, err error) bool {
	if strings.Contains(err.Error(), "Something went wrong while executing your query") || strings.Contains(err.Error(), "EOF") {
		sdk.LogInfo(logger, "retryable error detected, will pause for about one minute", "err", err)
		control.Paused(time.Now().Add(time.Minute))
		time.Sleep(time.Minute + time.Millisecond*time.Duration(rand.Int63n(500)))
		control.Resumed()
		sdk.LogInfo(logger, "retryable error resumed")
		return true
	}
	return false
}

func (g *GithubIntegration) checkForAbuseDetection(logger sdk.Logger, control sdk.Control, err error) bool {
	// first check our retry-after since we get better resolution on how much to slow down
	if ok, retry := sdk.IsRateLimitError(err); ok {
		sdk.LogInfo(logger, "rate limit detected", "until", time.Now().Add(retry))
		control.Paused(time.Now().Add(retry))
		time.Sleep(retry)
		control.Resumed()
		sdk.LogInfo(logger, "rate limit wake up")
		return true
	}
	if strings.Contains(err.Error(), "You have triggered an abuse detection mechanism") {
		// we need to try and back off at least 1min + some randomized number of additional ms
		sdk.LogInfo(logger, "abuse detection, will pause for about one minute")
		control.Paused(time.Now().Add(time.Minute))
		time.Sleep(time.Minute + time.Millisecond*time.Duration(rand.Int63n(500)))
		control.Resumed()
		sdk.LogInfo(logger, "abuse detection resumed")
		return true
	}
	return false
}

func (g *GithubIntegration) checkForRateLimit(logger sdk.Logger, control sdk.Control, rateLimit rateLimit) error {
	// check for rate limit
	if rateLimit.ShouldPause() {
		if err := control.Paused(rateLimit.ResetAt); err != nil {
			return err
		}
		// pause until we are no longer rate limited
		sdk.LogInfo(logger, "rate limited", "until", rateLimit.ResetAt)
		time.Sleep(time.Until(rateLimit.ResetAt))
		sdk.LogInfo(logger, "rate limit wake up")
		// send a resume now that we're no longer rate limited
		if err := control.Resumed(); err != nil {
			return err
		}
	}
	sdk.LogDebug(logger, "rate limit detail", "remaining", rateLimit.Remaining, "cost", rateLimit.Cost, "total", rateLimit.Limit)
	return nil
}

func (g *GithubIntegration) fetchPullRequestCommits(logger sdk.Logger, client sdk.GraphQLClient, userManager *UserManager, control sdk.Control, customerID string, name string, pullRequestID string, repoID string, cursor string) ([]*sdk.SourceCodePullRequestCommit, error) {
	sdk.LogInfo(logger, "need to run a pull request paginated commits starting from "+cursor, "repo", name, "pullrequest_id", pullRequestID)
	after := cursor
	var variables = map[string]interface{}{
		"first": defaultPullRequestCommitPageSize,
		"id":    pullRequestID,
	}
	var retryCount int
	commits := make([]*sdk.SourceCodePullRequestCommit, 0)
	for {
		if after != "" {
			variables["after"] = after
		}
		sdk.LogDebug(logger, "running queued pullrequests export", "repo", name, "after", after, "limit", variables["first"], "retryCount", retryCount)
		var result pullrequestPagedCommitsResult
		g.lock.Lock() // just to prevent too many GH requests
		if err := client.Query(generateAllPRCommitsQuery("", after), variables, &result); err != nil {
			g.lock.Unlock()
			if g.checkForAbuseDetection(logger, control, err) {
				continue
			}
			if g.checkForRetryableError(logger, control, err) {
				retryCount++
				variables["first"] = defaultRetryPageSize // back off the page size to see if this will help
				if retryCount >= 10 {
					return nil, fmt.Errorf("failed to export pull request commits after retrying 10 times for %s", name)
				}
				continue
			}
			return nil, err
		}
		g.lock.Unlock()
		retryCount = 0
		for _, edge := range result.Node.Commits.Edges {
			prcommit, err := edge.Node.Commit.ToModel(logger, userManager, customerID, repoID, pullRequestID)
			if err != nil {
				return nil, err
			}
			commits = append(commits, prcommit)
		}
		if err := g.checkForRateLimit(logger, control, result.RateLimit); err != nil {
			return nil, err
		}
		if !result.Node.PageInfo.HasNextPage {
			break
		}
		after = result.Node.PageInfo.EndCursor
	}
	return commits, nil
}

func (g *GithubIntegration) queuePullRequestJob(logger sdk.Logger, client sdk.GraphQLClient, userManager *UserManager, repoOwner string, repoName string, repoID string, cursor string) job {
	return func(export sdk.Export, pipe sdk.Pipe) error {
		sdk.LogInfo(logger, "need to run a pull request job starting from "+cursor, "name", repoName, "owner", repoOwner)
		var variables = map[string]interface{}{
			"first": defaultPageSize,
			"after": cursor,
			"owner": repoOwner,
			"name":  repoName,
		}
		customerID := export.CustomerID()
		fullname := repoOwner + "/" + repoName
		var retryCount int
		for {
			sdk.LogDebug(logger, "running queued pullrequests export", "repo", fullname, "after", variables["after"], "limit", variables["first"], "retryCount", retryCount)
			var result repositoryPullrequests
			g.lock.Lock() // just to prevent too many GH requests
			if err := client.Query(pullrequestPagedQuery, variables, &result); err != nil {
				g.lock.Unlock()
				if g.checkForAbuseDetection(logger, export, err) {
					continue
				}
				if g.checkForRetryableError(logger, export, err) {
					retryCount++
					variables["first"] = defaultRetryPageSize // back off the page size to see if this will help
					if retryCount >= 10 {
						return fmt.Errorf("failed to export after retrying 10 times for %s", fullname)
					}
					continue
				}
				return err
			}
			g.lock.Unlock()
			retryCount = 0
			for _, predge := range result.Repository.Pullrequests.Edges {
				pullrequest, err := predge.Node.ToModel(logger, userManager, customerID, fullname, repoID)
				if err != nil {
					return fmt.Errorf("failed to convert pull request to model: %w", err)
				}
				for _, reviewedge := range predge.Node.Reviews.Edges {
					prreview, err := reviewedge.Node.ToModel(logger, userManager, customerID, repoID, pullrequest.ID)
					if err != nil {
						return err
					}
					if err := pipe.Write(prreview); err != nil {
						return err
					}
				}
				if predge.Node.Reviews.PageInfo.HasNextPage {
					job := g.queuePullRequestReviewsJob(logger, client, userManager, repoOwner, repoName, repoID, pullrequest.ID, predge.Node.Number, predge.Node.Reviews.PageInfo.EndCursor)
					if err := job(export, pipe); err != nil {
						return err
					}
				}
				commits := make([]*sdk.SourceCodePullRequestCommit, 0)
				for _, commitedge := range predge.Node.Commits.Edges {
					prcommit, err := commitedge.Node.Commit.ToModel(logger, userManager, customerID, repoID, pullrequest.ID)
					if err != nil {
						return err
					}
					commits = append(commits, prcommit)
				}
				if predge.Node.Commits.PageInfo.HasNextPage {
					// fetch all the remaining paged commits
					morecommits, err := g.fetchPullRequestCommits(logger, client, userManager, export, customerID, fullname, predge.Node.ID, pullrequest.RepoID, predge.Node.Commits.PageInfo.EndCursor)
					if err != nil {
						return err
					}
					commits = append(commits, morecommits...)
				}
				// set the commits back on the pull request
				setPullRequestCommits(pullrequest, commits)
				// stream out all our commits
				for _, commit := range commits {
					if err := pipe.Write(commit); err != nil {
						return err
					}
				}
				// write the pull request after above in case we needed to get additional objects
				if err := pipe.Write(pullrequest); err != nil {
					return err
				}
			}
			if !result.Repository.Pullrequests.PageInfo.HasNextPage {
				break
			}
			if err := g.checkForRateLimit(logger, export, result.RateLimit); err != nil {
				return err
			}
			variables["after"] = result.Repository.Pullrequests.PageInfo.EndCursor
		}
		return nil
	}
}

func (g *GithubIntegration) queuePullRequestCommentsJob(logger sdk.Logger, client sdk.GraphQLClient, userManager *UserManager, repoOwner string, repoName string, repoID string, prID string, prNumber int, cursor string) job {
	return func(export sdk.Export, pipe sdk.Pipe) error {
		sdk.LogInfo(logger, "need to run a pull request comments job starting from "+cursor, "name", repoName, "owner", repoOwner)
		var variables = map[string]interface{}{
			"first":  defaultPageSize,
			"after":  cursor,
			"owner":  repoOwner,
			"name":   repoName,
			"number": prNumber,
		}
		customerID := export.CustomerID()
		fullname := repoOwner + "/" + repoName
		var retryCount int
		for {
			sdk.LogDebug(logger, "running queued pullrequests comments export", "number", prID, "repo", fullname, "after", variables["after"], "limit", variables["first"], "retryCount", retryCount)
			var result struct {
				RateLimit  rateLimit `json:"rateLimit"`
				Repository struct {
					PullRequest struct {
						Comments pullrequestcomments `json:"comments"`
					} `json:"pullRequest"`
				} `json:"repository"`
			}
			g.lock.Lock() // just to prevent too many GH requests
			if err := client.Query(pullrequestCommentsPagedQuery, variables, &result); err != nil {
				g.lock.Unlock()
				if g.checkForAbuseDetection(logger, export, err) {
					continue
				}
				if g.checkForRetryableError(logger, export, err) {
					retryCount++
					variables["first"] = defaultRetryPageSize // back off the page size to see if this will help
					if retryCount >= 10 {
						return fmt.Errorf("failed to export after retrying 10 times for %s", fullname)
					}
					continue
				}
				return err
			}
			g.lock.Unlock()
			retryCount = 0
			for _, edge := range result.Repository.PullRequest.Comments.Edges {
				prcomment, err := edge.Node.ToModel(logger, userManager, customerID, repoID, prID)
				if err != nil {
					return err
				}
				if err := pipe.Write(prcomment); err != nil {
					return err
				}
			}
			if !result.Repository.PullRequest.Comments.PageInfo.HasNextPage {
				break
			}
			if err := g.checkForRateLimit(logger, export, result.RateLimit); err != nil {
				return err
			}
			variables["after"] = result.Repository.PullRequest.Comments.PageInfo.EndCursor
		}
		return nil
	}
}

func (g *GithubIntegration) queuePullRequestReviewsJob(logger sdk.Logger, client sdk.GraphQLClient, userManager *UserManager, repoOwner string, repoName string, repoID string, prID string, prNumber int, cursor string) job {
	return func(export sdk.Export, pipe sdk.Pipe) error {
		sdk.LogInfo(logger, "need to run a pull request reviews job starting from "+cursor, "name", repoName, "owner", repoOwner)
		var variables = map[string]interface{}{
			"first":  defaultPageSize,
			"after":  cursor,
			"owner":  repoOwner,
			"name":   repoName,
			"number": prNumber,
		}
		customerID := export.CustomerID()
		fullname := repoOwner + "/" + repoName
		var retryCount int
		for {
			sdk.LogDebug(logger, "running queued pullrequests reviews export", "number", prID, "repo", fullname, "after", variables["after"], "limit", variables["first"], "retryCount", retryCount)
			var result struct {
				RateLimit  rateLimit `json:"rateLimit"`
				Repository struct {
					PullRequest struct {
						Reviews pullrequestreviews `json:"reviews"`
					} `json:"pullRequest"`
				} `json:"repository"`
			}
			g.lock.Lock() // just to prevent too many GH requests
			if err := client.Query(pullrequestReviewsPagedQuery, variables, &result); err != nil {
				g.lock.Unlock()
				if g.checkForAbuseDetection(logger, export, err) {
					continue
				}
				if g.checkForRetryableError(logger, export, err) {
					retryCount++
					variables["first"] = defaultRetryPageSize // back off the page size to see if this will help
					if retryCount >= 10 {
						return fmt.Errorf("failed to export after retrying 10 times for %s", fullname)
					}
					continue
				}
				return err
			}
			g.lock.Unlock()
			retryCount = 0
			for _, edge := range result.Repository.PullRequest.Reviews.Edges {
				prreview, err := edge.Node.ToModel(logger, userManager, customerID, repoID, prID)
				if err != nil {
					return err
				}
				if err := pipe.Write(prreview); err != nil {
					return err
				}
			}
			if !result.Repository.PullRequest.Reviews.PageInfo.HasNextPage {
				break
			}
			if err := g.checkForRateLimit(logger, export, result.RateLimit); err != nil {
				return err
			}
			variables["after"] = result.Repository.PullRequest.Reviews.PageInfo.EndCursor
		}
		return nil
	}
}

func (g *GithubIntegration) fetchAllRepos(logger sdk.Logger, client sdk.GraphQLClient, export sdk.Export, login string, scope string) ([]repoName, error) {
	repos := make([]repoName, 0)
	var variables = map[string]interface{}{
		"first": defaultPageSize,
		"login": login,
	}
	var after string
	var retryCount int
	for {
		if after != "" {
			variables["after"] = after
		}
		sdk.LogDebug(logger, "running fetch all repos", "login", login, "after", after, "limit", variables["first"], "retryCount", retryCount)
		var result repoWithNameResult
		if err := client.Query(generateAllReposQuery(after, scope), variables, &result); err != nil {
			if g.checkForAbuseDetection(logger, export, err) {
				continue
			}
			if g.checkForRetryableError(logger, export, err) {
				retryCount++
				variables["first"] = defaultRetryPageSize // back off the page size to see if this will help
				if retryCount >= 10 {
					return nil, fmt.Errorf("failed to fetch repos after retrying 10 times for %s (%s)", login, scope)
				}
				continue
			}
			return nil, err
		}
		retryCount = 0
		for _, repo := range result.Data.Repositories.Nodes {
			repos = append(repos, repo)
		}
		if err := g.checkForRateLimit(logger, export, result.RateLimit); err != nil {
			return nil, err
		}
		if !result.Data.Repositories.PageInfo.HasNextPage {
			break
		}
		after = result.Data.Repositories.PageInfo.EndCursor
	}
	return repos, nil
}

func (g *GithubIntegration) fetchViewer(logger sdk.Logger, client sdk.GraphQLClient, export sdk.Export) (string, error) {
	var retryCount int
	for {
		sdk.LogDebug(logger, "running viewer query", "retryCount", retryCount)
		var result viewerResult
		if err := client.Query(generateViewerLogin(), nil, &result); err != nil {
			if g.checkForAbuseDetection(logger, export, err) {
				continue
			}
			if g.checkForRetryableError(logger, export, err) {
				retryCount++
				continue
			}
			return "", err
		}
		retryCount = 0
		return result.Viewer.Login, nil
	}
}

func (g *GithubIntegration) getRepoKey(name string) string {
	return fmt.Sprintf("repo_cursor_%s", name)
}

func (g *GithubIntegration) fetchRepos(logger sdk.Logger, client sdk.GraphQLClient, export sdk.Export, repos []string) ([]repository, error) {
	results := make([]repository, 0)
	var retryCount int
	var offset int
	const max = 10
	state := export.State()
	for offset < len(repos) {
		sdk.LogDebug(logger, "running repo query", "retryCount", retryCount, "offset", offset, "length", len(repos))
		result := make(map[string]json.RawMessage)
		var sb strings.Builder
		end := offset + max
		if end > len(repos) {
			end = len(repos)
		}
		// concat multiple parallel queries for each repo
		for i, repo := range repos[offset:end] {
			tok := strings.Split(repo, "/")
			owner := tok[0]
			name := tok[1]
			label := fmt.Sprintf("repo%d", i)
			var cursor string
			if !export.Historical() {
				state.Get(g.getRepoKey(repo), &cursor)
				sb.WriteString(getAllRepoDataQuery(owner, name, label, cursor))
			}
		}
		if err := client.Query("query { "+sb.String()+" rateLimit { limit cost remaining resetAt } }", nil, &result); err != nil {
			if g.checkForAbuseDetection(logger, export, err) {
				continue
			}
			if g.checkForRetryableError(logger, export, err) {
				retryCount++
				continue
			}
			return nil, err
		}
		for key, buf := range result {
			if key == "rateLimit" {
				var rl rateLimit
				if err := easyjson.Unmarshal(buf, &rl); err != nil {
					return nil, err
				}
				if err := g.checkForRateLimit(logger, export, rl); err != nil {
					return nil, err
				}
			} else {
				var repo repository
				if err := easyjson.Unmarshal(buf, &repo); err != nil {
					return nil, err
				}
				results = append(results, repo)
			}
		}
		retryCount = 0
		offset += len(repos)
	}
	return results, nil
}

func (g *GithubIntegration) newGraphClient(logger sdk.Logger, config sdk.Config) (string, sdk.GraphQLClient, error) {
	url := "https://api.github.com/graphql"

	var client sdk.GraphQLClient

	if config.APIKeyAuth != nil {
		apikey := config.APIKeyAuth.APIKey
		if config.APIKeyAuth.URL != "" {
			url = config.APIKeyAuth.URL
		}
		client = g.manager.GraphQLManager().New(url, map[string]string{
			"Authorization": "bearer " + apikey,
		})
		sdk.LogInfo(logger, "using apikey authorization")
	} else if config.OAuth2Auth != nil {
		authToken := config.OAuth2Auth.AccessToken
		if config.OAuth2Auth.RefreshToken != nil {
			token, err := g.manager.RefreshOAuth2Token(refType, *config.OAuth2Auth.RefreshToken)
			if err != nil {
				return "", nil, fmt.Errorf("error refreshing oauth2 access token: %w", err)
			}
			authToken = token
		}
		if config.OAuth2Auth.URL != "" {
			url = config.OAuth2Auth.URL
		}
		client = g.manager.GraphQLManager().New(url, map[string]string{
			"Authorization": "bearer " + authToken,
		})
		sdk.LogInfo(logger, "using oauth2 authorization")
	} else if config.BasicAuth != nil {
		if config.BasicAuth.URL != "" {
			url = config.BasicAuth.URL
		}
		client = g.manager.GraphQLManager().New(url, map[string]string{
			"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte(config.BasicAuth.Username+":"+config.BasicAuth.Password)),
		})
		sdk.LogInfo(logger, "using basic authorization", "username", config.BasicAuth.Username)
	} else {
		return "", nil, fmt.Errorf("supported authorization not provided. support for: apikey, oauth2, basic")
	}
	return url, client, nil
}

func (g *GithubIntegration) newHTTPClient(logger sdk.Logger, config sdk.Config) (string, sdk.HTTPClient, error) {
	url := "https://api.github.com/"

	var client sdk.HTTPClient

	if config.APIKeyAuth != nil {
		apikey := config.APIKeyAuth.APIKey
		if config.APIKeyAuth.URL != "" {
			url = config.APIKeyAuth.URL
		}
		client = g.manager.HTTPManager().New(url, map[string]string{
			"Authorization": "bearer " + apikey,
		})
		sdk.LogInfo(logger, "using apikey authorization")
	} else if config.OAuth2Auth != nil {
		authToken := config.OAuth2Auth.AccessToken
		if config.OAuth2Auth.RefreshToken != nil {
			token, err := g.manager.RefreshOAuth2Token(refType, *config.OAuth2Auth.RefreshToken)
			if err != nil {
				return "", nil, fmt.Errorf("error refreshing oauth2 access token: %w", err)
			}
			authToken = token
		}
		if config.OAuth2Auth.URL != "" {
			url = config.OAuth2Auth.URL
		}
		client = g.manager.HTTPManager().New(url, map[string]string{
			"Authorization": "bearer " + authToken,
		})
		sdk.LogInfo(logger, "using oauth2 authorization")
	} else if config.BasicAuth != nil {
		if config.BasicAuth.URL != "" {
			url = config.BasicAuth.URL
		}
		client = g.manager.HTTPManager().New(url, map[string]string{
			"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte(config.BasicAuth.Username+":"+config.BasicAuth.Password)),
		})
		sdk.LogInfo(logger, "using basic authorization", "username", config.BasicAuth.Username)
	} else {
		return "", nil, fmt.Errorf("supported authorization not provided. support for: apikey, oauth2, basic")
	}
	return url, client, nil
}

// Export is called to tell the integration to run an export
func (g *GithubIntegration) Export(export sdk.Export) error {
	logger := sdk.LogWith(g.logger, "customer_id", export.CustomerID(), "job_id", export.JobID())
	sdk.LogInfo(logger, "export started")
	pipe := export.Pipe()
	config := export.Config()

	url, client, err := g.newGraphClient(logger, config)
	if err != nil {
		return fmt.Errorf("error creating graphql client: %w", err)
	}

	_, httpclient, err := g.newHTTPClient(logger, config)
	if err != nil {
		return fmt.Errorf("error creating http client: %w", err)
	}

	sdk.LogInfo(logger, "export starting", "url", url)

	// TODO: add skip public repos since we're going to have a specific customer_id (empty) to do those in the future

	var orgs []string
	var users []string
	if config.Accounts == nil {
		// first we're going to fetch all the organizations that the viewer is a member of if accounts if nil
		var allorgs allOrgsResult
		for {
			if err := client.Query(allOrgsQuery, map[string]interface{}{"first": 100}, &allorgs); err != nil {
				if g.checkForAbuseDetection(logger, export, err) {
					continue
				}
				return err
			}
			for _, node := range allorgs.Viewer.Organizations.Nodes {
				if node.IsMember {
					orgs = append(orgs, node.Login)
				} else {
					sdk.LogInfo(logger, "skipping "+node.Login+" the authorized user is not a member of this org")
				}
			}
			break
		}
		viewer, err := g.fetchViewer(logger, client, export)
		if err != nil {
			return err
		}
		users = append(users, viewer)
	} else {
		for _, acct := range *config.Accounts {
			if acct.Type == sdk.ConfigAccountTypeOrg {
				orgs = append(orgs, acct.ID)
			} else {
				users = append(users, acct.ID)
			}
		}
	}

	includeRepo := func(login string, name string, isArchived bool) bool {
		if config.Exclusions != nil && config.Exclusions.Matches(login, name) {
			// skip any repos that don't match our rule
			sdk.LogInfo(logger, "skipping repo because it matched exclusion rule", "name", name)
			return false
		}
		if config.Inclusions != nil && !config.Inclusions.Matches(login, name) {
			// skip any repos that don't match our rule
			sdk.LogInfo(logger, "skipping repo because it didn't match inclusion rule", "name", name)
			return false
		}
		return isArchived == false
	}

	sdk.LogDebug(logger, "exporting the following accounts", "orgs", orgs, "users", users)

	repos := make(map[string]repoName, 0)
	reponames := make([]string, 0)

	// add all the user repos
	for _, login := range users {
		userrepos, err := g.fetchAllRepos(logger, client, export, login, "user")
		if err != nil {
			return fmt.Errorf("error fetching all repos for user %s: %w", login, err)
		}
		for _, repo := range userrepos {
			if includeRepo(login, repo.Name, repo.IsArchived) {
				repo.Scope = sdk.ConfigAccountTypeUser
				repo.Login = login
				repos[repo.Name] = repo
				reponames = append(reponames, repo.Name)
			}
		}
	}

	// add all the org repos
	for _, login := range orgs {
		orgrepos, err := g.fetchAllRepos(logger, client, export, login, "organization")
		if err != nil {
			return fmt.Errorf("error fetching all repos for org %s: %w", login, err)
		}
		for _, repo := range orgrepos {
			if includeRepo(login, repo.Name, repo.IsArchived) {
				repo.Scope = sdk.ConfigAccountTypeOrg
				repo.Login = login
				repos[repo.Name] = repo
				reponames = append(reponames, repo.Name)
			}
		}
	}

	// fetch the repo data to include all the related entities like pull requests etc
	therepos, err := g.fetchRepos(logger, client, export, reponames)
	if err != nil {
		return fmt.Errorf("error fetching repos: %w", err)
	}

	customerID := export.CustomerID()
	instanceID := export.IntegrationInstanceID()
	userManager := NewUserManager(customerID, orgs, export, pipe, g, instanceID)
	jobs := make([]job, 0)
	started := time.Now()
	state := export.State()
	var repoCount, prCount, reviewCount, commitCount, commentCount int
	var hasPreviousRepos bool
	previousRepos := make(map[string]*sdk.SourceCodeRepo)

	if state.Exists(previousReposStateKey) {
		if _, err := state.Get(previousReposStateKey, &previousRepos); err != nil {
			sdk.LogError(logger, "error fetching previous repos state", "err", err)
		} else {
			hasPreviousRepos = true
		}
	}

	if hasPreviousRepos {
		// make all the repos in this batch so we can see if any of the previous weren't
		reposFound := make(map[string]bool)
		for _, node := range therepos {
			reposFound[node.Name] = true
		}
		for _, repo := range previousRepos {
			// if not found, it may be that we're now excluding it OR
			// it could mean that the repo has been deleted/removed
			// in either case we need to mark the repo as inactive
			if !reposFound[repo.Name] {
				repo.Active = false
				repo.UpdatedAt = datetime.EpochNow()
				sdk.LogInfo(logger, "deactivating a repo no longer processed", "name", repo.Name)
				if err := pipe.Write(repo); err != nil {
					return err
				}
				// remove the webhook
				r := repos[repo.Name]
				g.uninstallRepoWebhook(state, httpclient, r.Login, instanceID, repo.Name)
			}
		}
	}

	for _, node := range therepos {
		sdk.LogInfo(logger, "processing repo: "+node.Name, "id", node.ID)

		repoCount++
		r := repos[node.Name]

		if err := g.installRepoWebhookIfRequired(customerID, logger, state, httpclient, r.Login, instanceID, node.Name); err != nil {
			return err
		}

		repo := node.ToModel(customerID, instanceID, r.Login, r.IsPrivate, r.Scope)
		previousRepos[node.Name] = repo // remember it
		if err := pipe.Write(repo); err != nil {
			return err
		}
		for _, predge := range node.Pullrequests.Edges {
			pullrequest, err := predge.Node.ToModel(logger, userManager, customerID, repo.Name, repo.ID)
			if err != nil {
				return fmt.Errorf("failed to convert pull request to model: %w", err)
			}
			for _, reviewedge := range predge.Node.Reviews.Edges {
				prreview, err := reviewedge.Node.ToModel(logger, userManager, customerID, repo.ID, pullrequest.ID)
				if err != nil {
					return err
				}
				if err := pipe.Write(prreview); err != nil {
					return fmt.Errorf("error fetching review for pull request %s for repo: %v. %w", pullrequest.ID, r.Name, err)
				}
				reviewCount++
			}
			if predge.Node.Reviews.PageInfo.HasNextPage {
				jobs = append(jobs, g.queuePullRequestReviewsJob(logger, client, userManager, r.Login, r.RepoName, repo.GetID(), pullrequest.ID, predge.Node.Number, predge.Node.Comments.PageInfo.EndCursor))
			}
			for _, commentedge := range predge.Node.Comments.Edges {
				prcomment, err := commentedge.Node.ToModel(logger, userManager, customerID, repo.ID, pullrequest.ID)
				if err != nil {
					return err
				}
				if err := pipe.Write(prcomment); err != nil {
					return fmt.Errorf("error fetching comment for pull request %s for repo: %v. %w", pullrequest.ID, r.Name, err)
				}
				commentCount++
			}
			if predge.Node.Comments.PageInfo.HasNextPage {
				jobs = append(jobs, g.queuePullRequestCommentsJob(logger, client, userManager, r.Login, r.RepoName, repo.GetID(), pullrequest.ID, predge.Node.Number, predge.Node.Comments.PageInfo.EndCursor))
			}
			commits := make([]*sdk.SourceCodePullRequestCommit, 0)
			for _, commitedge := range predge.Node.Commits.Edges {
				prcommit, err := commitedge.Node.Commit.ToModel(logger, userManager, customerID, repo.ID, pullrequest.ID)
				if err != nil {
					return err
				}
				commits = append(commits, prcommit)
			}
			if predge.Node.Commits.PageInfo.HasNextPage {
				// fetch all the remaining paged commits
				morecommits, err := g.fetchPullRequestCommits(logger, client, userManager, export, customerID, repo.Name, predge.Node.ID, pullrequest.RepoID, predge.Node.Commits.PageInfo.EndCursor)
				if err != nil {
					return fmt.Errorf("error fetching commits for pull request %s for repo: %v. %w", pullrequest.ID, r.Name, err)
				}
				commits = append(commits, morecommits...)
				sdk.LogDebug(logger, "fetched pull request commits", "count", len(commits), "pullrequest_id", predge.Node.ID, "repo", repo.Name)
			}
			// set the commits back on the pull request
			setPullRequestCommits(pullrequest, commits)
			// stream out all our commits
			for _, commit := range commits {
				if err := pipe.Write(commit); err != nil {
					return err
				}
				commitCount++
			}
			// stream out our pullrequest
			if err := pipe.Write(pullrequest); err != nil {
				return err
			}
			prCount++
		}
		// save off where we started at so we can page from there in subsequent exports
		if err := state.Set(g.getRepoKey(repo.Name), node.Pullrequests.PageInfo.StartCursor); err != nil {
			return fmt.Errorf("error saving repo state: %w", err)
		}
		if node.Pullrequests.PageInfo.HasNextPage {
			// queue the pull requests for the next page
			jobs = append(jobs, g.queuePullRequestJob(logger, client, userManager, r.Login, r.RepoName, repo.GetID(), node.Pullrequests.PageInfo.EndCursor))
		}
	}

	// remember the repos we processed
	if err := state.Set(previousReposStateKey, previousRepos); err != nil {
		return fmt.Errorf("error saving previous repos state: %w", err)
	}

	sdk.LogInfo(logger, "initial export completed", "duration", time.Since(started), "repoCount", repoCount, "prCount", prCount, "reviewCount", reviewCount, "commitCount", commitCount, "commentCount", commentCount)

	_, skipHistorical := g.config.GetBool("skip-historical")

	if !skipHistorical {

		// flush any pending data to get it to send immediately
		pipe.Flush()

		// now cycle through any pending jobs after the first pass
		var wg sync.WaitGroup
		var maxSize = 2
		jobch := make(chan job, maxSize*5)
		errors := make(chan error, maxSize)
		// run our jobs in parallel but we're going to run the graphql request in single threaded mode to try
		// and reduce abuse from GitHub but at least the processing can be done parallel on our side
		for i := 0; i < maxSize; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for job := range jobch {
					if err := job(export, pipe); err != nil {
						sdk.LogError(logger, "error running job", "err", err)
						errors <- err
						return
					}
					// docs say a min of one second between requests
					// https://developer.github.com/v3/guides/best-practices-for-integrators/#dealing-with-abuse-rate-limits
					time.Sleep(time.Second)
				}
			}()
		}
		for _, job := range jobs {
			jobch <- job
		}
		// close and wait for all our jobs to complete
		close(jobch)
		wg.Wait()
		// check to see if we had an early exit
		select {
		case err := <-errors:
			return err
		default:
		}
	}

	return nil
}
