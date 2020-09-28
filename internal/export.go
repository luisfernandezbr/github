package internal

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/url"
	"strings"
	"sync"
	"time"

	easyjson "github.com/mailru/easyjson"
	"github.com/pinpt/agent/v4/sdk"
)

const (
	defaultPageSize                  = 50
	defaultRetryPageSize             = 25
	defaultPullRequestCommitPageSize = 100
	previousReposStateKey            = "previous_repos"
	previousProjectsStateKey         = "previous_projects"
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

func (g *GithubIntegration) queuePullRequestJob(logger sdk.Logger, client sdk.GraphQLClient, userManager *UserManager, repoName string, repoID string, cursor string) job {
	repoOwner, repoLogin := g.getRepoDetails(repoName)
	return func(export sdk.Export, pipe sdk.Pipe) error {
		sdk.LogInfo(logger, "need to run a pull request job starting from "+cursor, "name", repoName, "owner", repoOwner)
		var variables = map[string]interface{}{
			"first": defaultPageSize,
			"after": cursor,
			"owner": repoOwner,
			"name":  repoLogin,
		}
		customerID := export.CustomerID()
		var retryCount int
		for {
			sdk.LogDebug(logger, "running queued pullrequests export", "repo", repoName, "after", variables["after"], "limit", variables["first"], "retryCount", retryCount)
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
						return fmt.Errorf("failed to export after retrying 10 times for %s", repoName)
					}
					continue
				}
				return err
			}
			g.lock.Unlock()
			retryCount = 0
			for _, predge := range result.Repository.Pullrequests.Edges {
				pullrequest, err := predge.Node.ToModel(logger, userManager, customerID, repoName, repoID)
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
					job := g.queuePullRequestReviewsJob(logger, client, userManager, repoName, repoID, pullrequest.ID, predge.Node.Number, predge.Node.Reviews.PageInfo.EndCursor)
					if err := job(export, pipe); err != nil {
						return err
					}
				}
				for _, reviewreqedge := range predge.Node.ReviewRequests.Edges {
					prreviewrequest, err := reviewreqedge.Node.ToModel(logger, userManager, customerID, repoID, pullrequest.ID, predge.Node.UpdatedAt)
					if err != nil {
						return err
					}
					if err := pipe.Write(prreviewrequest); err != nil {
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
					morecommits, err := g.fetchPullRequestCommits(logger, client, userManager, export, customerID, repoName, predge.Node.ID, pullrequest.RepoID, predge.Node.Commits.PageInfo.EndCursor)
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

func (g *GithubIntegration) queuePullRequestCommentsJob(logger sdk.Logger, client sdk.GraphQLClient, userManager *UserManager, repoName string, repoID string, prID string, prNumber int, cursor string) job {
	repoOwner, repoLogin := g.getRepoDetails(repoName)
	return func(export sdk.Export, pipe sdk.Pipe) error {
		sdk.LogInfo(logger, "need to run a pull request comments job starting from "+cursor, "name", repoName, "owner", repoOwner)
		var variables = map[string]interface{}{
			"first":  defaultPageSize,
			"after":  cursor,
			"owner":  repoOwner,
			"name":   repoLogin,
			"number": prNumber,
		}
		customerID := export.CustomerID()
		var retryCount int
		for {
			sdk.LogDebug(logger, "running queued pullrequests comments export", "number", prID, "repo", repoName, "after", variables["after"], "limit", variables["first"], "retryCount", retryCount)
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
						return fmt.Errorf("failed to export after retrying 10 times for %s", repoName)
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

func (g *GithubIntegration) queuePullRequestReviewsJob(logger sdk.Logger, client sdk.GraphQLClient, userManager *UserManager, repoName string, repoID string, prID string, prNumber int, cursor string) job {
	repoOwner, repoLogin := g.getRepoDetails(repoName)
	return func(export sdk.Export, pipe sdk.Pipe) error {
		sdk.LogInfo(logger, "need to run a pull request reviews job starting from "+cursor, "name", repoName)
		var variables = map[string]interface{}{
			"first":  defaultPageSize,
			"owner":  repoOwner,
			"name":   repoLogin,
			"number": prNumber,
		}
		if cursor != "" {
			variables["after"] = cursor
		}
		customerID := export.CustomerID()
		var retryCount int
		for {
			sdk.LogDebug(logger, "running queued pullrequests reviews export", "number", prID, "repo", repoName, "after", variables["after"], "limit", variables["first"], "retryCount", retryCount)
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
						return fmt.Errorf("failed to export after retrying 10 times for %s", repoName)
					}
					continue
				}
				return fmt.Errorf("error fetching pull request reviews: %w", err)
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
			sdk.LogDebug(logger, "found a repo", "name", repo.Name)
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
	sdk.LogDebug(logger, "returning from fetch all repos", "count", len(repos))
	return repos, nil
}

func (g *GithubIntegration) fetchViewer(logger sdk.Logger, client sdk.GraphQLClient, export sdk.Control) (*viewer, error) {
	var retryCount int
	for {
		sdk.LogDebug(logger, "running viewer query", "retryCount", retryCount)
		var result viewerResult
		if err := client.Query(viewerQuery, nil, &result); err != nil {
			if g.checkForAbuseDetection(logger, export, err) {
				continue
			}
			if g.checkForRetryableError(logger, export, err) {
				retryCount++
				continue
			}
			return nil, err
		}
		retryCount = 0
		return &result.Viewer, nil
	}
}

// fetchOrgs will fetch all orgs this user is a member of
func (g *GithubIntegration) fetchOrgs(logger sdk.Logger, client sdk.GraphQLClient, export sdk.Control) ([]org, error) {
	var allorgs allOrgsResult
	var orgs []org
	for {
		if err := client.Query(allOrgsQuery, map[string]interface{}{"first": 100}, &allorgs); err != nil {
			if g.checkForAbuseDetection(logger, export, err) {
				continue
			}
			return nil, err
		}
		for _, node := range allorgs.Viewer.Organizations.Nodes {
			if node.IsMember {
				orgs = append(orgs, node)
			} else {
				sdk.LogInfo(logger, "skipping "+node.Login+" the authorized user is not a member of this org")
			}
		}
		break
	}
	return orgs, nil
}

func (g *GithubIntegration) fetchAllRepoMilestones(logger sdk.Logger, client sdk.GraphQLClient, userManager *UserManager, export sdk.Export, repoName, repoRefID string, historical bool) error {
	repoOwner, repoLogin := g.getRepoDetails(repoName)
	var variables = map[string]interface{}{
		"owner": repoOwner,
		"name":  repoLogin,
	}
	var after, before string
	var retryCount int
	var first string
	customerID := export.CustomerID()
	integrationInstanceID := export.IntegrationInstanceID()
	projectID := sdk.NewWorkProjectID(customerID, repoRefID, refType)
	pipe := export.Pipe()
	state := export.State()
	state.Get("milestones_"+repoName, &before)
	if before != "" {
		variables["before"] = before
	}
	for {
		if after != "" {
			variables["after"] = after
			delete(variables, "before")
		}
		sdk.LogDebug(logger, "running fetch all repo milestones", "name", repoName, "login", repoLogin, "after", after, "limit", variables["first"], "retryCount", retryCount)
		var result repositoryMilestonesResult
		if err := client.Query(repositoryMilestonesQuery, variables, &result); err != nil {
			if g.checkForAbuseDetection(logger, export, err) {
				continue
			}
			if g.checkForRetryableError(logger, export, err) {
				continue
			}
			return err
		}
		retryCount = 0
		for _, node := range result.Repository.Milestones.Nodes {
			issue, err := node.ToModel(logger, userManager, customerID, integrationInstanceID, repoName, projectID)
			if err != nil {
				return err
			}
			if issue != nil {
				if err := pipe.Write(issue); err != nil {
					return err
				}
			}
		}
		if err := g.checkForRateLimit(logger, export, result.RateLimit); err != nil {
			return err
		}
		if first == "" {
			first = result.Repository.Milestones.PageInfo.StartCursor
		}
		if !result.Repository.Milestones.PageInfo.HasNextPage {
			break
		}
		after = result.Repository.Milestones.PageInfo.EndCursor
	}
	if first != "" {
		return state.Set("milestones_"+repoName, first)
	}
	return nil
}

func (g *GithubIntegration) getRepoDetails(repoName string) (string, string) {
	tok := strings.Split(repoName, "/")
	return tok[0], tok[1]
}

func (g *GithubIntegration) fetchAllRepoIssues(logger sdk.Logger, client sdk.GraphQLClient, userManager *UserManager, export sdk.Export, repoName, repoRefID string, historical bool) error {
	repoOwner, repoLogin := g.getRepoDetails(repoName)
	var variables = map[string]interface{}{
		"owner": repoOwner,
		"name":  repoLogin,
	}
	var after, before string
	var retryCount int
	var first string
	customerID := export.CustomerID()
	integrationInstanceID := export.IntegrationInstanceID()
	projectID := sdk.NewWorkProjectID(customerID, repoRefID, refType)
	pipe := export.Pipe()
	state := export.State()
	state.Get("issues_"+repoName, &before)
	if before != "" {
		variables["before"] = before
	}
	for {
		if after != "" {
			variables["after"] = after
			delete(variables, "before")
		}
		sdk.LogDebug(logger, "running fetch all repo issues", "name", repoName, "login", repoLogin, "after", after, "limit", variables["first"], "retryCount", retryCount)
		var result issueResult
		if err := client.Query(issuesQuery, variables, &result); err != nil {
			if g.checkForAbuseDetection(logger, export, err) {
				continue
			}
			if g.checkForRetryableError(logger, export, err) {
				continue
			}
			return err
		}
		retryCount = 0
		for _, node := range result.Repository.Issues.Nodes {
			issue, err := node.ToModel(logger, userManager, customerID, integrationInstanceID, repoName, projectID)
			if err != nil {
				return err
			}
			if issue != nil {
				if err := pipe.Write(issue); err != nil {
					return err
				}
				for _, c := range node.Comments.Nodes {
					comment, err := c.ToModel(logger, userManager, customerID, integrationInstanceID, projectID, issue.ID)
					if err != nil {
						return err
					}
					if err := pipe.Write(comment); err != nil {
						return err
					}
				}
			}
		}
		if err := g.checkForRateLimit(logger, export, result.RateLimit); err != nil {
			return err
		}
		if first == "" {
			first = result.Repository.Issues.PageInfo.StartCursor
		}
		if !result.Repository.Issues.PageInfo.HasNextPage {
			break
		}
		after = result.Repository.Issues.PageInfo.EndCursor
	}
	if first != "" {
		return state.Set("issues_"+repoName, first)
	}
	return nil
}

func (g *GithubIntegration) fetchRepoProject(logger sdk.Logger, client sdk.GraphQLClient, pipe sdk.Pipe, control sdk.Control, customerID, integrationInstanceID, repoName, repoRefID string, num int) error {
	repoOwner, repoLogin := g.getRepoDetails(repoName)
	var retryCount int
	variables := map[string]interface{}{
		"owner": repoOwner,
		"name":  repoLogin,
		"num":   num,
	}
	for {
		sdk.LogDebug(logger, "running repo project query", "retryCount", retryCount, "num", num, "name", repoName)
		var result repoProjectResult
		if err := client.Query(repoProjectQuery, variables, &result); err != nil {
			if g.checkForAbuseDetection(logger, control, err) {
				continue
			}
			if g.checkForRetryableError(logger, control, err) {
				retryCount++
				continue
			}
			return err
		}
		projectID := sdk.NewWorkProjectID(customerID, repoRefID, refType)
		b, p := result.Repository.Project.ToModel(logger, customerID, integrationInstanceID, projectID)
		if b != nil {
			sdk.LogDebug(logger, "writing repo board", "name", repoName)
			if err := pipe.Write(b); err != nil {
				return err
			}
		}
		if p != nil {
			sdk.LogDebug(logger, "writing repo project", "name", repoName)
			if err := pipe.Write(p); err != nil {
				return err
			}
		}
		retryCount = 0
		return nil
	}
}

func (g *GithubIntegration) fetchRepoProjects(logger sdk.Logger, client sdk.GraphQLClient, export sdk.Export, repoName, repoRefID string) error {
	repoOwner, repoLogin := g.getRepoDetails(repoName)
	var retryCount int
	variables := map[string]interface{}{
		"owner": repoOwner,
		"name":  repoLogin,
	}
	for {
		sdk.LogDebug(logger, "running repo project query", "retryCount", retryCount, "name", repoName)
		var result repoProjectsResult
		if err := client.Query(repoProjectsQuery, variables, &result); err != nil {
			if g.checkForAbuseDetection(logger, export, err) {
				continue
			}
			if g.checkForRetryableError(logger, export, err) {
				retryCount++
				continue
			}
			return err
		}
		for _, project := range result.Repository.Projects.Nodes {
			projectID := sdk.NewWorkProjectID(export.CustomerID(), repoRefID, refType)
			b, p := project.ToModel(logger, export.CustomerID(), export.IntegrationInstanceID(), projectID)
			if b != nil {
				sdk.LogDebug(logger, "writing repo board", "name", project.Name)
				if err := export.Pipe().Write(b); err != nil {
					return err
				}
			}
			if p != nil {
				sdk.LogDebug(logger, "writing repo project", "name", project.Name)
				if err := export.Pipe().Write(p); err != nil {
					return err
				}
			}
		}
		retryCount = 0
		return nil
	}
}

func (g *GithubIntegration) getRepoKey(name string) string {
	return fmt.Sprintf("repo_cursor_%s", name)
}

func (g *GithubIntegration) fetchRepos(logger sdk.Logger, client sdk.GraphQLClient, export sdk.Export, repos []string) ([]repository, error) {
	results := make([]repository, 0)
	var retryCount int
	var offset int
	const max = 5
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
			}
			sb.WriteString(getAllRepoDataQuery(owner, name, label, cursor))
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
				offset++
			}
		}
		retryCount = 0
	}
	sdk.LogInfo(logger, "returning from fetchRepos", "len", len(results))
	return results, nil
}

// https://docs.github.com/en/graphql/overview/schema-previews

var previewHeaders = []string{
	"application/vnd.github.package-deletes-preview+json",
	"application/vnd.github.flash-preview+json",
	"application/vnd.github.antiope-preview+json",
	"application/vnd.github.starfox-preview+json",
	"application/vnd.github.bane-preview+json",
	"application/vnd.github.stone-crop-preview+json",
	"application/vnd.github.nebula-preview+json",
	"application/vnd.github.shadow-cat-preview+json",
	"application/vnd.github.starfire-preview+json",
	"application/json",
	"*/*",
}

func (g *GithubIntegration) getHeaders(headers map[string]string) map[string]string {
	headers["Accept"] = strings.Join(previewHeaders, ", ")
	return headers
}

func (g *GithubIntegration) getGraphqlURL(theurl string) string {
	u, _ := url.Parse(theurl)
	u.Path = "/graphql"
	return u.String()
}

func (g *GithubIntegration) newGraphClient(logger sdk.Logger, config sdk.Config) (string, sdk.GraphQLClient, error) {
	url := "https://api.github.com/graphql"

	var client sdk.GraphQLClient

	if config.APIKeyAuth != nil {
		apikey := config.APIKeyAuth.APIKey
		if config.APIKeyAuth.URL != "" {
			url = g.getGraphqlURL(config.APIKeyAuth.URL)
		}
		client = g.manager.GraphQLManager().New(url, g.getHeaders(
			map[string]string{
				"Authorization": "bearer " + apikey,
			}),
		)
		sdk.LogInfo(logger, "using apikey authorization")
	} else if config.OAuth2Auth != nil {
		authToken := config.OAuth2Auth.AccessToken
		if config.OAuth2Auth.RefreshToken != nil && *config.OAuth2Auth.RefreshToken != "" {
			token, err := g.manager.AuthManager().RefreshOAuth2Token(refType, *config.OAuth2Auth.RefreshToken)
			if err != nil {
				return "", nil, fmt.Errorf("error refreshing oauth2 access token: %w", err)
			}
			authToken = token
		}
		if config.OAuth2Auth.URL != "" {
			url = g.getGraphqlURL(config.OAuth2Auth.URL)
		}
		client = g.manager.GraphQLManager().New(url, g.getHeaders(
			map[string]string{
				"Authorization": "bearer " + authToken,
			}),
		)
		sdk.LogInfo(logger, "using oauth2 authorization")
	} else if config.BasicAuth != nil {
		if config.BasicAuth.URL != "" {
			url = g.getGraphqlURL(config.BasicAuth.URL)
		}
		client = g.manager.GraphQLManager().New(url, g.getHeaders(
			map[string]string{
				"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte(config.BasicAuth.Username+":"+config.BasicAuth.Password)),
			}),
		)
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
		client = g.manager.HTTPManager().New(url, g.getHeaders(
			map[string]string{
				"Authorization": "bearer " + apikey,
			}),
		)
		sdk.LogInfo(logger, "using apikey authorization", "url", url)
	} else if config.OAuth2Auth != nil {
		authToken := config.OAuth2Auth.AccessToken
		if config.OAuth2Auth.RefreshToken != nil && *config.OAuth2Auth.RefreshToken != "" {
			token, err := g.manager.AuthManager().RefreshOAuth2Token(refType, *config.OAuth2Auth.RefreshToken)
			if err != nil {
				return "", nil, fmt.Errorf("error refreshing oauth2 access token: %w", err)
			}
			authToken = token
		}
		if config.OAuth2Auth.URL != "" {
			url = config.OAuth2Auth.URL
		}
		client = g.manager.HTTPManager().New(url, g.getHeaders(
			map[string]string{
				"Authorization": "bearer " + authToken,
			}),
		)
		sdk.LogInfo(logger, "using oauth2 authorization", "url", url)
	} else if config.BasicAuth != nil {
		if config.BasicAuth.URL != "" {
			url = config.BasicAuth.URL
		}
		client = g.manager.HTTPManager().New(url, g.getHeaders(
			map[string]string{
				"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte(config.BasicAuth.Username+":"+config.BasicAuth.Password)),
			}),
		)
		sdk.LogInfo(logger, "using basic authorization", "username", config.BasicAuth.Username, "url", url)
	} else {
		sdk.LogDebug(logger, "config JSON: "+sdk.Stringify(config))
		return "", nil, fmt.Errorf("supported authorization not provided. support for: apikey, oauth2, basic")
	}
	return url, client, nil
}

// Export is called to tell the integration to run an export
func (g *GithubIntegration) Export(export sdk.Export) error {
	logger := sdk.LogWith(g.logger, "customer_id", export.CustomerID(), "job_id", export.JobID())
	sdk.LogInfo(logger, "export started", "historical", export.Historical())
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
		fullorgs, err := g.fetchOrgs(logger, client, export)
		if err != nil {
			return fmt.Errorf("error fetching orgs: %w", err)
		}
		for _, org := range fullorgs {
			orgs = append(orgs, org.Login)
		}
		viewer, err := g.fetchViewer(logger, client, export)
		if err != nil {
			return err
		}
		users = append(users, viewer.Login)
	} else {
		for _, acct := range *config.Accounts {
			if acct.Selected != nil && !*acct.Selected {
				continue
			}
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
		if isArchived {
			sdk.LogInfo(logger, "skipping repo because it is archived", "name", name)
			return false
		}
		return true
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
				sdk.LogInfo(logger, "user repo will be included", "name", repo.Name, "login", login)
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
				sdk.LogInfo(logger, "org repo will be included", "name", repo.Name, "login", login)
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
	state := export.State()
	userManager := NewUserManager(customerID, orgs, export, state, pipe, g, instanceID, export.Historical())
	jobs := make([]job, 0)
	started := time.Now()
	var repoCount, prCount, reviewCount, reviewRequestCount, commitCount, commentCount int
	var hasPreviousRepos bool
	previousRepos := make(map[string]*sdk.SourceCodeRepo)
	previousProjects := make(map[string]*sdk.WorkProject)

	if state.Exists(previousReposStateKey) {
		if _, err := state.Get(previousReposStateKey, &previousRepos); err != nil {
			sdk.LogError(logger, "error fetching previous repos state", "err", err)
		} else {
			hasPreviousRepos = len(previousRepos) > 0
		}
	}
	if state.Exists(previousProjectsStateKey) {
		if _, err := state.Get(previousReposStateKey, &previousProjects); err != nil {
			sdk.LogError(logger, "error fetching previous projects state", "err", err)
		}
	}

	if hasPreviousRepos {
		// make all the repos in this batch so we can see if any of the previous weren't
		reposFound := make(map[string]bool)
		for _, node := range therepos {
			reposFound[node.Name] = true
			sdk.LogDebug(logger, "processing repo", "name", node.Name, "ref_id", node.ID)
		}
		for _, repo := range previousRepos {
			// if not found, it may be that we're now excluding it OR
			// it could mean that the repo has been deleted/removed
			// in either case we need to mark the repo as inactive
			if !reposFound[repo.Name] {
				repo.Active = false
				repo.UpdatedAt = sdk.EpochNow()
				sdk.LogInfo(logger, "deactivating a repo no longer processed", "name", repo.Name)
				if err := pipe.Write(repo); err != nil {
					return err
				}
				// remove the webhook
				r := repos[repo.Name]
				g.uninstallRepoWebhook(g.manager.WebHookManager(), httpclient, customerID, instanceID, r.Login, repo.Name, r.ID)
				// deactivate the project as well if one exists
				project := previousProjects[repo.ID]
				if project != nil {
					project.Active = false
					project.UpdatedAt = sdk.EpochNow()
					sdk.LogInfo(logger, "deactivating a project no longer processed", "name", repo.Name)
					if err := pipe.Write(project); err != nil {
						return err
					}
				}
			}
		}
	}

	// process the work config
	if err := g.processWorkConfig(config, pipe, state, customerID, instanceID, export.Historical()); err != nil {
		return fmt.Errorf("error processing work config: %w", err)
	}

	if err := g.processDefaultIssueType(logger, pipe, state, customerID, instanceID, export.Historical()); err != nil {
		return fmt.Errorf("error processing default issue type: %w", err)
	}

	for _, node := range therepos {
		sdk.LogInfo(logger, "processing repo: "+node.Name, "id", node.ID)

		repoCount++
		r := repos[node.Name]

		hookInstalled, err := g.installRepoWebhookIfRequired(g.manager.WebHookManager(), logger, httpclient, customerID, instanceID, r.Login, r.Name, r.ID)
		if err != nil {
			return err
		}

		repo, project, capability := node.ToModel(export.State(), export.Historical(), customerID, instanceID, r.Login, r.IsPrivate, r.Scope)

		previousRepos[node.Name] = repo // remember it
		if project != nil {
			previousProjects[repo.ID] = project
		}

		if hookInstalled && !export.Historical() {
			// if the hook is installed this isn't a historical, we can skip processing this repo
			sdk.LogDebug(logger, "skipping repo since a webhook is already installed and not historical", "name", node.Name, "id", node.ID)
			continue
		}
		if err := pipe.Write(repo); err != nil {
			return err
		}
		if project != nil {
			if err := pipe.Write(project); err != nil {
				return err
			}
		}
		if capability != nil {
			if err := pipe.Write(capability); err != nil {
				return err
			}
		}

		// write out any labels as issue types
		for _, labelnode := range node.Labels.Nodes {
			o, err := labelnode.ToModel(logger, state, customerID, instanceID, export.Historical())
			if err != nil {
				return err
			}
			if o != nil {
				if err := pipe.Write(o); err != nil {
					return err
				}
			}
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
				jobs = append(jobs, g.queuePullRequestReviewsJob(logger, client, userManager, r.Name, repo.GetID(), pullrequest.ID, predge.Node.Number, predge.Node.Reviews.PageInfo.EndCursor))
			}
			for _, reviewRequestedge := range predge.Node.ReviewRequests.Edges {
				prreview, err := reviewRequestedge.Node.ToModel(logger, userManager, customerID, repo.ID, pullrequest.ID, predge.Node.UpdatedAt)
				if err != nil {
					return err
				}
				if err := pipe.Write(prreview); err != nil {
					return fmt.Errorf("error writing review request for pull request %s for repo: %v. %w", pullrequest.ID, r.Name, err)
				}
				reviewRequestCount++
			}
			if predge.Node.ReviewRequests.PageInfo.HasNextPage {
				// TODO(robin): queue job if has nextpage, for prs with >10 reviewers requested
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
				jobs = append(jobs, g.queuePullRequestCommentsJob(logger, client, userManager, r.Name, repo.GetID(), pullrequest.ID, predge.Node.Number, predge.Node.Comments.PageInfo.EndCursor))
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

		if r.HasIssuesEnabled {
			sdk.LogDebug(logger, "issues enabled for this repo", "name", node.Name)
			if err := g.fetchAllRepoIssues(logger, client, userManager, export, r.Name, r.ID, export.Historical()); err != nil {
				return fmt.Errorf("error fetching repo issues: %w", err)
			}
			if err := g.fetchAllRepoMilestones(logger, client, userManager, export, r.Name, r.ID, export.Historical()); err != nil {
				return fmt.Errorf("error fetching repo milestones: %w", err)
			}
		}

		if project != nil && r.HasProjectsEnabled {
			sdk.LogDebug(logger, "projects enabled for this repo", "name", node.Name)
			if err := g.fetchRepoProjects(logger, client, export, r.Name, r.ID); err != nil {
				return fmt.Errorf("error fetching repo projects: %w", err)
			}
		}

		// save off where we started at so we can page from there in subsequent exports
		if err := state.Set(g.getRepoKey(repo.Name), node.Pullrequests.PageInfo.StartCursor); err != nil {
			return fmt.Errorf("error saving repo state: %w", err)
		}
		if node.Pullrequests.PageInfo.HasNextPage {
			// queue the pull requests for the next page
			jobs = append(jobs, g.queuePullRequestJob(logger, client, userManager, r.Name, repo.GetID(), node.Pullrequests.PageInfo.EndCursor))
		}
	}

	// remember the repos and projects we processed
	if err := state.Set(previousReposStateKey, previousRepos); err != nil {
		return fmt.Errorf("error saving previous repos state: %w", err)
	}
	if err := state.Set(previousProjectsStateKey, previousProjects); err != nil {
		return fmt.Errorf("error saving previous projects state: %w", err)
	}
	sdk.LogDebug(logger, "saved previous state", "repos", len(previousRepos), "projects", len(previousProjects))

	sdk.LogInfo(logger, "initial export completed", "duration", time.Since(started), "repoCount", repoCount, "prCount", prCount, "reviewCount", reviewCount, "reviewRequestCount", reviewRequestCount, "commitCount", commitCount, "commentCount", commentCount, "jobs", len(jobs))

	_, skipHistorical := g.config.GetBool("skip-historical")

	if !skipHistorical && len(jobs) > 0 {

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
