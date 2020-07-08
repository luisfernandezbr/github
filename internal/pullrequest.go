package internal

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/go-github/v32/github"
	"github.com/pinpt/agent.next/sdk"
)

type pullrequestTimelineItems struct {
	Nodes []author
}

type pullrequest struct {
	ID            string                   `json:"id"`
	Body          string                   `json:"bodyHTML"`
	URL           string                   `json:"url"`
	Closed        bool                     `json:"closed"`
	Draft         bool                     `json:"draft"`
	Locked        bool                     `json:"locked"`
	Merged        bool                     `json:"merged"`
	Number        int                      `json:"number"`
	State         string                   `json:"state"`
	Title         string                   `json:"title"`
	CreatedAt     time.Time                `json:"createdAt"`
	UpdatedAt     time.Time                `json:"updatedAt"`
	MergedAt      time.Time                `json:"mergedAt"`
	Author        author                   `json:"author"`
	Branch        string                   `json:"branch"`
	MergeCommit   oidProp                  `json:"mergeCommit"`
	MergedBy      author                   `json:"mergedBy"`
	Commits       pullrequestcommits       `json:"commits"`
	Reviews       pullrequestreviews       `json:"reviews"`
	Comments      pullrequestcomments      `json:"comments"`
	TimelineItems pullrequestTimelineItems `json:"timelineItems"`
}

func (g *GithubIntegration) fromPullRequestEvent(logger sdk.Logger, client sdk.GraphQLClient, userManager *UserManager, control sdk.Control, customerID string, pr *github.PullRequestEvent) (*sdk.SourceCodePullRequest, error) {
	var action string
	if pr.Action != nil {
		action = *pr.Action
	}
	switch action {
	case "opened", "synchronize", "edited", "ready_for_review", "locked", "unlocked", "reopened", "closed", "converted_to_draft":
		var object pullrequest
		object.ID = *pr.PullRequest.NodeID
		if pr.PullRequest.Body != nil {
			object.Body = toHTML(*pr.PullRequest.Body)
		}
		object.URL = *pr.PullRequest.HTMLURL
		if action == "closed" {
			// If the action is "closed" and the "merged" key is "false", the pull request was closed with unmerged commits.
			// If the action is "closed" and the "merged" key is "true", the pull request was merged.
			object.Closed = true
			object.TimelineItems = pullrequestTimelineItems{
				Nodes: []author{userToAuthor(pr.Sender)},
			}
		}
		object.Author = userToAuthor(pr.PullRequest.User)
		object.Draft = *pr.PullRequest.Draft
		object.Locked = *pr.PullRequest.Locked
		object.Merged = *pr.PullRequest.Merged
		object.Number = *pr.PullRequest.Number
		object.State = strings.ToUpper(*pr.PullRequest.State)
		object.Title = *pr.PullRequest.Title
		object.CreatedAt = *pr.PullRequest.CreatedAt
		if pr.PullRequest.UpdatedAt != nil {
			object.UpdatedAt = *pr.PullRequest.UpdatedAt
		}
		if pr.PullRequest.MergedAt != nil {
			object.MergedAt = *pr.PullRequest.MergedAt
		}
		object.Branch = *pr.PullRequest.Head.Ref
		object.MergeCommit = oidProp{*pr.PullRequest.Base.SHA}
		repoID := sdk.NewSourceCodeRepoID(customerID, *pr.Repo.NodeID, refType)
		result, err := object.ToModel(logger, userManager, customerID, *pr.Repo.FullName, repoID)
		if err != nil {
			return nil, err
		}
		commits, err := g.fetchPullRequestCommits(logger, client, userManager, control, customerID, *pr.Repo.FullName, *pr.PullRequest.NodeID, repoID, "")
		if err != nil {
			return nil, err
		}
		setPullRequestCommits(result, commits)
		return result, nil
	default:
		sdk.LogInfo(logger, "unhandled pull request action: "+action)
	}
	return nil, nil
}

func setPullRequestCommits(pullrequest *sdk.SourceCodePullRequest, commits []*sdk.SourceCodePullRequestCommit) {
	commitids := []string{}
	commitshas := []string{}
	// commits come from Github in the latest to earliest
	for i := len(commits) - 1; i >= 0; i-- {
		commit := commits[i]
		commitshas = append(commitshas, commit.Sha)
		commitids = append(commitids, sdk.NewSourceCodeCommitID(pullrequest.CustomerID, commit.Sha, refType, pullrequest.RepoID))
	}
	pullrequest.CommitShas = commitshas
	pullrequest.CommitIds = commitids
	if len(commitids) > 0 {
		pullrequest.BranchID = sdk.NewSourceCodeBranchID(refType, pullrequest.RepoID, pullrequest.CustomerID, pullrequest.BranchName, pullrequest.CommitIds[0])
	} else {
		pullrequest.BranchID = sdk.NewSourceCodeBranchID(refType, pullrequest.RepoID, pullrequest.CustomerID, pullrequest.BranchName, "")
	}
	for _, commit := range commits {
		commit.BranchID = pullrequest.BranchID
	}
}

func (pr pullrequest) ToModel(logger sdk.Logger, userManager *UserManager, customerID string, repoName string, repoID string) (*sdk.SourceCodePullRequest, error) {
	pullrequest := &sdk.SourceCodePullRequest{}
	pullrequest.ID = sdk.NewSourceCodePullRequestID(customerID, pr.ID, refType, repoID)
	pullrequest.CustomerID = customerID
	pullrequest.RepoID = repoID
	pullrequest.RefID = pr.ID
	pullrequest.RefType = refType
	pullrequest.Title = pr.Title
	pullrequest.URL = pr.URL
	pullrequest.Description = pr.Body
	pullrequest.Draft = pr.Draft
	pullrequest.Active = true
	pullrequest.CreatedByRefID = pr.Author.RefID(customerID)
	if err := userManager.emitAuthor(logger, pr.Author); err != nil {
		return nil, err
	}
	pullrequest.BranchName = pr.Branch
	pullrequest.IntegrationInstanceID = sdk.StringPointer(userManager.instanceid)
	pullrequest.Identifier = fmt.Sprintf("%s#%d", repoName, pr.Number)
	if pr.Merged {
		pullrequest.MergeSha = pr.MergeCommit.Oid
		pullrequest.MergeCommitID = sdk.NewSourceCodeCommitID(customerID, pr.MergeCommit.Oid, refType, repoID)
		sdk.ConvertTimeToDateModel(pr.MergedAt, &pullrequest.MergedDate)
		pullrequest.MergedByRefID = pr.MergedBy.RefID(customerID)
		if err := userManager.emitAuthor(logger, pr.MergedBy); err != nil {
			return nil, err
		}
	}
	sdk.ConvertTimeToDateModel(pr.CreatedAt, &pullrequest.CreatedDate)
	sdk.ConvertTimeToDateModel(pr.UpdatedAt, &pullrequest.UpdatedDate)
	switch pr.State {
	case "OPEN":
		if pr.Locked {
			pullrequest.Status = sdk.SourceCodePullRequestStatusLocked
		} else {
			pullrequest.Status = sdk.SourceCodePullRequestStatusOpen
		}
	case "CLOSED":
		pullrequest.Status = sdk.SourceCodePullRequestStatusClosed
		if len(pr.TimelineItems.Nodes) > 0 {
			if err := userManager.emitAuthor(logger, pr.TimelineItems.Nodes[0]); err != nil {
				return nil, err
			}
			pullrequest.ClosedByRefID = pr.TimelineItems.Nodes[0].RefID(customerID)
		}
		sdk.ConvertTimeToDateModel(pr.UpdatedAt, &pullrequest.ClosedDate)
	case "MERGED":
		pullrequest.Status = sdk.SourceCodePullRequestStatusMerged
	}
	return pullrequest, nil
}

func (g *GithubIntegration) updatePullrequest(logger sdk.Logger, config sdk.Config, id string, mutation *sdk.SourcecodePullRequestUpdateMutation, user sdk.MutationUser) error {
	payload := make(map[string]interface{})
	if mutation.Set.Title != nil {
		payload["title"] = *mutation.Set.Title
	}
	if mutation.Set.Description != nil {
		md, err := sdk.ConvertHTMLToMarkdown(*mutation.Set.Description)
		if err != nil {
			return fmt.Errorf("not able to transform body from HTML to Markdown: %w", err)
		}
		payload["body"] = md
	}
	if mutation.Set.Status != nil {
		payload["state"] = *mutation.Set.Status
	}
	if len(payload) == 0 {
		return fmt.Errorf("the mutation failed because invalid value was passed: %s", sdk.Stringify(mutation))
	}
	payload["pullRequestId"] = id
	var c sdk.Config // copy in the config for the user
	c.APIKeyAuth = user.APIKeyAuth
	c.BasicAuth = user.BasicAuth
	c.OAuth2Auth = user.OAuth2Auth
	_, client, err := g.newGraphClient(logger, c)
	if err != nil {
		return fmt.Errorf("error creating http client: %w", err)
	}
	var resp mutationResponse
	sdk.LogDebug(logger, "sending pull request mutation", "input", payload, "user", user.ID)
	if err := client.Query(pullRequestUpdateMutation, map[string]interface{}{"input": payload}, &resp); err != nil {
		return err
	}
	// TODO: if webhook not enabled, we want to pull the PR result and ingest into pipe
	return nil
}

type repositoryPullrequests struct {
	Repository repository `json:"repository"`
	RateLimit  rateLimit  `json:"rateLimit"`
}

type pullrequestNode struct {
	Cursor string      `json:"cursor"`
	Node   pullrequest `json:"node"`
}

type pullrequests struct {
	TotalCount int               `json:"totalCount"`
	PageInfo   pageInfo          `json:"pageInfo"`
	Edges      []pullrequestNode `json:"edges"`
}

type pullrequestPagedCommit struct {
	Commit pullrequestCommit `json:"commit"`
}
type pullrequestPagedCommitNode struct {
	Node pullrequestPagedCommit `json:"node"`
}

type pullrequestPagedCommitEdges struct {
	Cursor string                       `json:"cursor"`
	Edges  []pullrequestPagedCommitNode `json:"edges"`
}

type pullrequestPagedCommits struct {
	TotalCount int                         `json:"totalCount"`
	PageInfo   pageInfo                    `json:"pageInfo"`
	Commits    pullrequestPagedCommitEdges `json:"commits"`
}

type pullrequestPagedCommitsResult struct {
	RateLimit rateLimit               `json:"rateLimit"`
	Node      pullrequestPagedCommits `json:"node"`
}
