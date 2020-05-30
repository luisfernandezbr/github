package internal

import (
	"math/rand"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/pinpt/agent.next/sdk"
	"github.com/pinpt/go-common/log"
)

const defaultPageSize = 100

type job func(export sdk.Export, pipe sdk.Pipe) error

func (g *GithubIntegration) checkForAbuseDetection(export sdk.Export, err error) bool {
	// first check our retry-after since we get better resolution on how much to slow down
	if ok, retry := sdk.IsRateLimitError(err); ok {
		export.Paused(time.Now().Add(retry))
		time.Sleep(retry)
		export.Resumed()
		return true
	}
	if strings.Contains(err.Error(), "You have triggered an abuse detection mechanism") {
		// we need to try and back off at least 1min + some randomized number of additional ms
		export.Paused(time.Now().Add(time.Minute))
		time.Sleep(time.Minute + time.Millisecond*time.Duration(rand.Int63n(500)))
		export.Resumed()
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
		log.Debug(g.logger, "rate limited", "until", rateLimit.ResetAt)
		time.Sleep(time.Until(rateLimit.ResetAt))
		log.Debug(g.logger, "rate limit wake up")
		// send a resume now that we're no longer rate limited
		if err := export.Resumed(); err != nil {
			return err
		}
	}
	log.Debug(g.logger, "rate limit detail", "remaining", rateLimit.Remaining, "cost", rateLimit.Cost, "total", rateLimit.Limit)
	return nil
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
		for {
			var result repositoryPullrequests
			g.lock.Lock() // just to prevent too many GH requests
			if err := g.client.Query(pullrequestPageQuery, variables, &result); err != nil {
				g.lock.Unlock()
				if g.checkForAbuseDetection(export, err) {
					continue
				}
				return err
			}
			g.lock.Unlock()
			for _, prnode := range result.Repository.Pullrequests.Nodes {
				pullrequest := prnode.ToModel(export.CustomerID(), repoOwner+"/"+repoName, repoID)
				if err := pipe.Write(pullrequest); err != nil {
					return err
				}
				for _, reviewnode := range prnode.Reviews.Nodes {
					prreview := reviewnode.ToModel(export.CustomerID(), repoID, pullrequest.GetID())
					if err := pipe.Write(prreview); err != nil {
						return err
					}
				}
				if prnode.Reviews.PageInfo.HasNextPage {
					// TODO: queue
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
	org := g.config["organization"]
	var variables = map[string]interface{}{
		"login": org,
		"first": 10,
	}
	jobs := make([]job, 0)
	started := time.Now()
	var repoCount, prCount, reviewCount int
	for {
		var result allQueryResult
		if err := g.client.Query(allDataQuery, variables, &result); err != nil {
			if g.checkForAbuseDetection(export, err) {
				continue
			}
			export.Completed(err)
			return nil
		}
		for _, node := range result.Organization.Repositories.Nodes {
			repoCount++
			repo := node.ToModel(export.CustomerID())
			if err := pipe.Write(repo); err != nil {
				return err
			}
			for _, prnode := range node.Pullrequests.Nodes {
				pullrequest := prnode.ToModel(export.CustomerID(), node.Name, repo.GetID())
				if err := pipe.Write(pullrequest); err != nil {
					return err
				}
				prCount++
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
	}
	log.Info(g.logger, "initial export completed", "duration", time.Since(started), "repoCount", repoCount, "prCount", prCount, "reviewCount", reviewCount)

	// now cycle through any pending jobs after the first pass
	var wg sync.WaitGroup
	var maxSize = runtime.NumCPU()
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
