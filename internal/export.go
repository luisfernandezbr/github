package internal

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/pinpt/agent.next/sdk"
	"github.com/pinpt/go-common/log"
	"github.com/pinpt/integration-sdk/sourcecode"
)

const (
	defaultPageSize                  = 50
	defaultRetryPageSize             = 25
	defaultPullRequestCommitPageSize = 100
)

type job func(export sdk.Export, pipe sdk.Pipe) error

func (g *GithubIntegration) checkForRetryableError(export sdk.Export, err error) bool {
	if strings.Contains(err.Error(), "Something went wrong while executing your query") {
		log.Info(g.logger, "retryable error detected, will pause for about one minute", "err", err)
		export.Paused(time.Now().Add(time.Minute))
		time.Sleep(time.Minute + time.Millisecond*time.Duration(rand.Int63n(500)))
		export.Resumed()
		log.Info(g.logger, "retryable error resumed")
		return true
	}
	return false
}

func (g *GithubIntegration) checkForAbuseDetection(export sdk.Export, err error) bool {
	// first check our retry-after since we get better resolution on how much to slow down
	if ok, retry := sdk.IsRateLimitError(err); ok {
		log.Info(g.logger, "rate limit detected", "until", time.Now().Add(retry))
		export.Paused(time.Now().Add(retry))
		time.Sleep(retry)
		export.Resumed()
		log.Info(g.logger, "rate limit wake up")
		return true
	}
	if strings.Contains(err.Error(), "You have triggered an abuse detection mechanism") {
		// we need to try and back off at least 1min + some randomized number of additional ms
		log.Info(g.logger, "abuse detection, will pause for about one minute")
		export.Paused(time.Now().Add(time.Minute))
		time.Sleep(time.Minute + time.Millisecond*time.Duration(rand.Int63n(500)))
		export.Resumed()
		log.Info(g.logger, "abuse detection resumed")
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
		log.Info(g.logger, "rate limited", "until", rateLimit.ResetAt)
		time.Sleep(time.Until(rateLimit.ResetAt))
		log.Info(g.logger, "rate limit wake up")
		// send a resume now that we're no longer rate limited
		if err := export.Resumed(); err != nil {
			return err
		}
	}
	log.Debug(g.logger, "rate limit detail", "remaining", rateLimit.Remaining, "cost", rateLimit.Cost, "total", rateLimit.Limit)
	return nil
}

func (g *GithubIntegration) fetchPullRequestCommits(export sdk.Export, name string, pullRequestID string, repoID string, branchID string, cursor string) ([]*sourcecode.PullRequestCommit, error) {
	log.Info(g.logger, "need to run a pull request paginated commits starting from "+cursor, "repo", name, "pullrequest_id", pullRequestID)
	var variables = map[string]interface{}{
		"first": defaultPullRequestCommitPageSize,
		"after": cursor,
		"id":    pullRequestID,
	}
	var retryCount int
	commits := make([]*sourcecode.PullRequestCommit, 0)
	for {
		log.Debug(g.logger, "running queued pullrequests export", "repo", name, "after", variables["after"], "limit", variables["first"], "retryCount", retryCount)
		var result pullrequestPagedCommitsResult
		g.lock.Lock() // just to prevent too many GH requests
		if err := g.client.Query(allPRCommitsQuery, variables, &result); err != nil {
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
		for _, node := range result.Node.Commits.Nodes {
			prcommit := node.Commit.ToModel(export.CustomerID(), repoID, branchID)
			commits = append(commits, prcommit)
		}
		if err := g.checkForRateLimit(export, result.RateLimit); err != nil {
			return nil, err
		}
		if !result.Node.PageInfo.HasNextPage {
			break
		}
		variables["after"] = result.Node.PageInfo.EndCursor
	}
	return commits, nil
}

func (g *GithubIntegration) queuePullRequestJob(repoOwner string, repoName string, repoID string, cursor string) job {
	return func(export sdk.Export, pipe sdk.Pipe) error {
		log.Info(g.logger, "need to run a pull request job starting from "+cursor, "name", repoName, "owner", repoOwner)
		var variables = map[string]interface{}{
			"first": defaultPageSize,
			"after": cursor,
			"owner": repoOwner,
			"name":  repoName,
		}
		fullname := repoOwner + "/" + repoName
		var retryCount int
		for {
			log.Debug(g.logger, "running queued pullrequests export", "repo", fullname, "after", variables["after"], "limit", variables["first"], "retryCount", retryCount)
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
			for _, prnode := range result.Repository.Pullrequests.Nodes {
				pullrequest := prnode.ToModel(export.CustomerID(), fullname, repoID)
				for _, reviewnode := range prnode.Reviews.Nodes {
					prreview := reviewnode.ToModel(export.CustomerID(), repoID, pullrequest.GetID())
					if err := pipe.Write(prreview); err != nil {
						return err
					}
				}
				if prnode.Reviews.PageInfo.HasNextPage {
					// TODO: queue
				}
				commits := make([]*sourcecode.PullRequestCommit, 0)
				for _, commitnode := range prnode.Commits.Nodes {
					prcommit := commitnode.Commit.ToModel(export.CustomerID(), repoID, pullrequest.BranchID)
					commits = append(commits, prcommit)
				}
				if prnode.Commits.PageInfo.HasNextPage {
					// fetch all the remaining paged commits
					morecommits, err := g.fetchPullRequestCommits(export, fullname, prnode.ID, pullrequest.RepoID, pullrequest.BranchID, prnode.Commits.PageInfo.EndCursor)
					if err != nil {
						return err
					}
					commits = append(commits, morecommits...)
				}
				// set the commits back on the pull request
				setCommits(pullrequest, commits)
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
	log.Info(g.logger, "export started")
	pipe, err := export.Start()
	if err != nil {
		return err
	}
	// first we're going to fetch all the organizations that the viewer is a member of
	var allorgs allOrgsResult
	for {
		if err := g.client.Query(allOrgsQuery, map[string]interface{}{"first": 100}, &allorgs); err != nil {
			if g.checkForAbuseDetection(export, err) {
				continue
			}
			export.Completed(err)
			return nil
		}
		break
	}
	var variables = map[string]interface{}{
		"first": 10,
	}
	jobs := make([]job, 0)
	started := time.Now()
	var repoCount, prCount, reviewCount, commitCount int
	for _, orgnode := range allorgs.Viewer.Organizations.Nodes {
		if !orgnode.IsMember {
			continue
		}
		variables["login"] = orgnode.Login
		var page, retryCount int
		for {
			log.Debug(g.logger, "running export", "org", orgnode.Login, "after", variables["after"], "page", page, "retryCount", retryCount)
			var result allQueryResult
			if err := g.client.Query(allDataQuery, variables, &result); err != nil {
				if g.checkForAbuseDetection(export, err) {
					continue
				}
				if g.checkForRetryableError(export, err) {
					retryCount++
					variables["first"] = 1 // back off the page size to see if this will help
					if retryCount >= 10 {
						return fmt.Errorf("failed to export after retrying 10 times for %s", orgnode.Login)
					}
					continue
				}
				export.Completed(err)
				return nil
			}
			retryCount = 0
			for _, node := range result.Organization.Repositories.Nodes {
				repoCount++
				repo := node.ToModel(export.CustomerID())
				if err := pipe.Write(repo); err != nil {
					return err
				}
				for _, prnode := range node.Pullrequests.Nodes {
					pullrequest := prnode.ToModel(export.CustomerID(), node.Name, repo.GetID())
					for _, reviewnode := range prnode.Reviews.Nodes {
						prreview := reviewnode.ToModel(export.CustomerID(), repo.GetID(), pullrequest.GetID())
						if err := pipe.Write(prreview); err != nil {
							return err
						}
						reviewCount++
					}
					if prnode.Reviews.PageInfo.HasNextPage {
						// TODO: queue
					}
					commits := make([]*sourcecode.PullRequestCommit, 0)
					for _, commitnode := range prnode.Commits.Nodes {
						prcommit := commitnode.Commit.ToModel(export.CustomerID(), repo.ID, pullrequest.BranchID)
						commits = append(commits, prcommit)
					}
					if prnode.Commits.PageInfo.HasNextPage {
						// fetch all the remaining paged commits
						morecommits, err := g.fetchPullRequestCommits(export, repo.Name, prnode.ID, pullrequest.RepoID, pullrequest.BranchID, prnode.Commits.PageInfo.EndCursor)
						if err != nil {
							return err
						}
						commits = append(commits, morecommits...)
						log.Debug(g.logger, "fetched pull request commits", "count", len(commits), "pullrequest_id", prnode.ID, "repo", repo.Name)
					}
					// set the commits back on the pull request
					setCommits(pullrequest, commits)
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
				if node.Pullrequests.PageInfo.HasNextPage {
					tok := strings.Split(node.Name, "/")
					// queue the pull requests for the next page
					jobs = append(jobs, g.queuePullRequestJob(tok[0], tok[1], repo.GetID(), node.Pullrequests.PageInfo.EndCursor))
				}
			}
			// check to see if we are at the end of our pagination
			if !result.Organization.Repositories.PageInfo.HasNextPage {
				break
			}
			if err := g.checkForRateLimit(export, result.RateLimit); err != nil {
				pipe.Close()
				export.Completed(err)
				return nil
			}
			variables["after"] = result.Organization.Repositories.PageInfo.EndCursor
			page++
		}
	}
	log.Info(g.logger, "initial export completed", "duration", time.Since(started), "repoCount", repoCount, "prCount", prCount, "reviewCount", reviewCount, "commitCount", commitCount)

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
					log.Error(g.logger, "error running job", "err", err)
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
		export.Completed(err)
		return nil
	default:
	}

	// finish it up
	if err := pipe.Close(); err != nil {
		return err
	}
	export.Completed(nil)
	return nil
}
