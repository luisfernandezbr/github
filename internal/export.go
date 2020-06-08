package internal

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/pinpt/agent.next/sdk"
)

const (
	defaultPageSize                  = 50
	defaultRetryPageSize             = 25
	defaultPullRequestCommitPageSize = 100
)

type job func(export sdk.Export, pipe sdk.Pipe) error

func (g *GithubIntegration) checkForRetryableError(export sdk.Export, err error) bool {
	if strings.Contains(err.Error(), "Something went wrong while executing your query") || strings.Contains(err.Error(), "EOF") {
		sdk.LogInfo(g.logger, "retryable error detected, will pause for about one minute", "err", err)
		export.Paused(time.Now().Add(time.Minute))
		time.Sleep(time.Minute + time.Millisecond*time.Duration(rand.Int63n(500)))
		export.Resumed()
		sdk.LogInfo(g.logger, "retryable error resumed")
		return true
	}
	return false
}

func (g *GithubIntegration) checkForAbuseDetection(export sdk.Export, err error) bool {
	// first check our retry-after since we get better resolution on how much to slow down
	if ok, retry := sdk.IsRateLimitError(err); ok {
		sdk.LogInfo(g.logger, "rate limit detected", "until", time.Now().Add(retry))
		export.Paused(time.Now().Add(retry))
		time.Sleep(retry)
		export.Resumed()
		sdk.LogInfo(g.logger, "rate limit wake up")
		return true
	}
	if strings.Contains(err.Error(), "You have triggered an abuse detection mechanism") {
		// we need to try and back off at least 1min + some randomized number of additional ms
		sdk.LogInfo(g.logger, "abuse detection, will pause for about one minute")
		export.Paused(time.Now().Add(time.Minute))
		time.Sleep(time.Minute + time.Millisecond*time.Duration(rand.Int63n(500)))
		export.Resumed()
		sdk.LogInfo(g.logger, "abuse detection resumed")
		return true
	}
	return false
}

func (g *GithubIntegration) checkForRateLimit(export sdk.Export, rateLimit rateLimit) error {
	// check for rate limit
	if rateLimit.ShouldPause() {
		if err := export.Paused(rateLimit.ResetAt); err != nil {
			return err
		}
		// pause until we are no longer rate limited
		sdk.LogInfo(g.logger, "rate limited", "until", rateLimit.ResetAt)
		time.Sleep(time.Until(rateLimit.ResetAt))
		sdk.LogInfo(g.logger, "rate limit wake up")
		// send a resume now that we're no longer rate limited
		if err := export.Resumed(); err != nil {
			return err
		}
	}
	sdk.LogDebug(g.logger, "rate limit detail", "remaining", rateLimit.Remaining, "cost", rateLimit.Cost, "total", rateLimit.Limit)
	return nil
}

func (g *GithubIntegration) fetchPullRequestCommits(userManager *UserManager, export sdk.Export, name string, pullRequestID string, repoID string, cursor string) ([]*sdk.SourceCodePullRequestCommit, error) {
	sdk.LogInfo(g.logger, "need to run a pull request paginated commits starting from "+cursor, "repo", name, "pullrequest_id", pullRequestID)
	after := cursor
	var variables = map[string]interface{}{
		"first": defaultPullRequestCommitPageSize,
		"id":    pullRequestID,
	}
	customerID := export.CustomerID()
	var retryCount int
	commits := make([]*sdk.SourceCodePullRequestCommit, 0)
	for {
		variables["after"] = after
		sdk.LogDebug(g.logger, "running queued pullrequests export", "repo", name, "after", after, "limit", variables["first"], "retryCount", retryCount)
		var result pullrequestPagedCommitsResult
		g.lock.Lock() // just to prevent too many GH requests
		if err := g.client.Query(generateAllPRCommitsQuery("", after), variables, &result); err != nil {
			g.lock.Unlock()
			if g.checkForAbuseDetection(export, err) {
				continue
			}
			if g.checkForRetryableError(export, err) {
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
			prcommit := edge.Node.Commit.ToModel(userManager, customerID, repoID, pullRequestID)
			commits = append(commits, prcommit)
		}
		if err := g.checkForRateLimit(export, result.RateLimit); err != nil {
			return nil, err
		}
		if !result.Node.PageInfo.HasNextPage {
			break
		}
		after = result.Node.PageInfo.EndCursor
	}
	return commits, nil
}

func (g *GithubIntegration) queuePullRequestJob(userManager *UserManager, repoOwner string, repoName string, repoID string, cursor string) job {
	return func(export sdk.Export, pipe sdk.Pipe) error {
		sdk.LogInfo(g.logger, "need to run a pull request job starting from "+cursor, "name", repoName, "owner", repoOwner)
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
			sdk.LogDebug(g.logger, "running queued pullrequests export", "repo", fullname, "after", variables["after"], "limit", variables["first"], "retryCount", retryCount)
			var result repositoryPullrequests
			g.lock.Lock() // just to prevent too many GH requests
			if err := g.client.Query(pullrequestPagedQuery, variables, &result); err != nil {
				g.lock.Unlock()
				if g.checkForAbuseDetection(export, err) {
					continue
				}
				if g.checkForRetryableError(export, err) {
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
				pullrequest := predge.Node.ToModel(userManager, customerID, fullname, repoID)
				for _, reviewedge := range predge.Node.Reviews.Edges {
					prreview := reviewedge.Node.ToModel(userManager, customerID, repoID, pullrequest.ID)
					if err := pipe.Write(prreview); err != nil {
						return err
					}
				}
				if predge.Node.Reviews.PageInfo.HasNextPage {
					// TODO: queue
				}
				commits := make([]*sdk.SourceCodePullRequestCommit, 0)
				for _, commitedge := range predge.Node.Commits.Edges {
					prcommit := commitedge.Node.Commit.ToModel(userManager, customerID, repoID, pullrequest.ID)
					commits = append(commits, prcommit)
				}
				if predge.Node.Commits.PageInfo.HasNextPage {
					// fetch all the remaining paged commits
					morecommits, err := g.fetchPullRequestCommits(userManager, export, fullname, predge.Node.ID, pullrequest.RepoID, predge.Node.Commits.PageInfo.EndCursor)
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
			if err := g.checkForRateLimit(export, result.RateLimit); err != nil {
				return err
			}
			variables["after"] = result.Repository.Pullrequests.PageInfo.EndCursor
		}
		return nil
	}
}

// Export is called to tell the integration to run an export
func (g *GithubIntegration) Export(export sdk.Export) error {
	sdk.LogInfo(g.logger, "export started")
	pipe, err := export.Pipe()
	if err != nil {
		return err
	}
	config := export.Config()
	ok, url := config.GetString("url")
	if !ok || url == "" {
		url = "https://api.github.com/graphql"
	}
	ok, apikey := config.GetString("api_key")
	if !ok {
		return fmt.Errorf("required api_key not found")
	}
	g.client = g.manager.GraphQLManager().New(url, map[string]string{
		"Authorization": "bearer " + apikey,
	})
	sdk.LogDebug(g.logger, "export starting", "url", url)

	// first we're going to fetch all the organizations that the viewer is a member of
	var allorgs allOrgsResult
	var orgs []string
	for {
		if err := g.client.Query(allOrgsQuery, map[string]interface{}{"first": 100}, &allorgs); err != nil {
			if g.checkForAbuseDetection(export, err) {
				continue
			}
			return err
		}
		for _, node := range allorgs.Viewer.Organizations.Nodes {
			if node.IsMember {
				orgs = append(orgs, node.Login)
			}
		}
		break
	}
	sdk.LogDebug(g.logger, "found organizations", "orgs", orgs)
	var variables = map[string]interface{}{
		"first": 10,
	}
	state := export.State()
	customerID := export.CustomerID()
	userManager := NewUserManager(customerID, orgs, export, pipe, g)
	jobs := make([]job, 0)
	started := time.Now()
	makeCursorKey := func(object string) string {
		return "cursor:" + object
	}
	// TODO: need to handle storing each object in state and then comparing on an incremental
	// for comments, commits, etc.
	var repoCount, prCount, reviewCount, commitCount, commentCount int
	var before, after string
	for _, login := range orgs {
		variables["login"] = login
		loginCursorKey := makeCursorKey("org_" + login)
		if _, err := state.Get(refType, loginCursorKey, &before); err != nil {
			return err
		}
		var firstCursor string
		var page, retryCount int
		for {
			if before != "" {
				variables["before"] = before
				delete(variables, "after")
			}
			if after != "" {
				variables["after"] = after
				delete(variables, "before")
			}
			sdk.LogDebug(g.logger, "running export", "org", login, "before", before, "after", after, "page", page, "retryCount", retryCount, "variables", variables)
			var result allQueryResult
			if err := g.client.Query(generateAllDataQuery(before, after), variables, &result); err != nil {
				if g.checkForAbuseDetection(export, err) {
					continue
				}
				if g.checkForRetryableError(export, err) {
					retryCount++
					variables["first"] = 1 // back off the page size to see if this will help
					if retryCount >= 10 {
						return fmt.Errorf("failed to export after retrying 10 times for %s", login)
					}
					continue
				}
				return err
			}
			if page == 0 {
				firstCursor = result.Organization.Repositories.PageInfo.StartCursor
			}
			retryCount = 0
			for _, edge := range result.Organization.Repositories.Edges {
				repoCount++
				repo := edge.Node.ToModel(customerID)
				if err := pipe.Write(repo); err != nil {
					return err
				}
				if edge.Node.IsArchived {
					// skip archived for now (but still send the repo)
					continue
				}
				for _, predge := range edge.Node.Pullrequests.Edges {
					pullrequest := predge.Node.ToModel(userManager, customerID, edge.Node.Name, repo.ID)
					for _, reviewedge := range predge.Node.Reviews.Edges {
						prreview := reviewedge.Node.ToModel(userManager, customerID, repo.ID, pullrequest.ID)
						if err := pipe.Write(prreview); err != nil {
							return err
						}
						reviewCount++
					}
					if predge.Node.Reviews.PageInfo.HasNextPage {
						// TODO: queue
					}
					for _, commentedge := range predge.Node.Comments.Edges {
						prcomment := commentedge.Node.ToModel(userManager, customerID, repo.ID, pullrequest.ID)
						if err := pipe.Write(prcomment); err != nil {
							return err
						}
						commentCount++
					}
					if predge.Node.Comments.PageInfo.HasNextPage {
						// TODO: queue
					}
					commits := make([]*sdk.SourceCodePullRequestCommit, 0)
					for _, commitedge := range predge.Node.Commits.Edges {
						prcommit := commitedge.Node.Commit.ToModel(userManager, customerID, repo.ID, pullrequest.ID)
						commits = append(commits, prcommit)
					}
					if predge.Node.Commits.PageInfo.HasNextPage {
						// fetch all the remaining paged commits
						morecommits, err := g.fetchPullRequestCommits(userManager, export, repo.Name, predge.Node.ID, pullrequest.RepoID, predge.Node.Commits.PageInfo.EndCursor)
						if err != nil {
							return err
						}
						commits = append(commits, morecommits...)
						sdk.LogDebug(g.logger, "fetched pull request commits", "count", len(commits), "pullrequest_id", predge.Node.ID, "repo", repo.Name)
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
				if edge.Node.Pullrequests.PageInfo.HasNextPage {
					tok := strings.Split(edge.Node.Name, "/")
					// queue the pull requests for the next page
					jobs = append(jobs, g.queuePullRequestJob(userManager, tok[0], tok[1], repo.GetID(), edge.Node.Pullrequests.PageInfo.EndCursor))
				}
			}
			// check to see if we are at the end of our pagination
			if !result.Organization.Repositories.PageInfo.HasNextPage {
				break
			}
			if err := g.checkForRateLimit(export, result.RateLimit); err != nil {
				pipe.Close()
				return err
			}
			after = result.Organization.Repositories.PageInfo.EndCursor
			before = ""
			page++
		}
		// set the first cursor so we can do a before on the next incremental
		sdk.LogDebug(g.logger, "setting the organization cursor", "org", login, "cursor", firstCursor)
		state.Set(refType, loginCursorKey, firstCursor)
	}
	sdk.LogInfo(g.logger, "initial export completed", "duration", time.Since(started), "repoCount", repoCount, "prCount", prCount, "reviewCount", reviewCount, "commitCount", commitCount, "commentCount", commentCount)

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
						sdk.LogError(g.logger, "error running job", "err", err)
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
			pipe.Close()
			return err
		default:
		}
	}

	// finish it up
	if err := pipe.Close(); err != nil {
		return err
	}
	return nil
}
