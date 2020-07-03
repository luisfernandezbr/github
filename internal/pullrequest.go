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

func userToAuthor(user *github.User) author {
	var author author
	if user.ID != nil {
		author.ID = user.GetNodeID()
	}
	author.Avatar = user.GetAvatarURL()
	author.Email = user.GetEmail()
	author.Login = user.GetLogin()
	author.Name = user.GetName()
	author.URL = user.GetHTMLURL()
	author.Type = "User"
	return author
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
		object.Body = *pr.PullRequest.Body // FIXME
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
		// TODO: do the remaining fields
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
	// FIXME: implement the remaining fields
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
		md, _ := sdk.NewDateWithTime(pr.MergedAt)
		pullrequest.MergedDate = sdk.SourceCodePullRequestMergedDate{
			Epoch:   md.Epoch,
			Rfc3339: md.Rfc3339,
			Offset:  md.Offset,
		}
		pullrequest.MergedByRefID = pr.MergedBy.RefID(customerID)
		if err := userManager.emitAuthor(logger, pr.MergedBy); err != nil {
			return nil, err
		}
	}
	cd, _ := sdk.NewDateWithTime(pr.CreatedAt)
	pullrequest.CreatedDate = sdk.SourceCodePullRequestCreatedDate{
		Epoch:   cd.Epoch,
		Rfc3339: cd.Rfc3339,
		Offset:  cd.Offset,
	}
	ud, _ := sdk.NewDateWithTime(pr.UpdatedAt)
	pullrequest.UpdatedDate = sdk.SourceCodePullRequestUpdatedDate{
		Epoch:   ud.Epoch,
		Rfc3339: ud.Rfc3339,
		Offset:  ud.Offset,
	}
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
		pullrequest.ClosedDate = sdk.SourceCodePullRequestClosedDate{
			Epoch:   ud.Epoch,
			Rfc3339: ud.Rfc3339,
			Offset:  ud.Offset,
		}
	case "MERGED":
		pullrequest.Status = sdk.SourceCodePullRequestStatusMerged
	}
	return pullrequest, nil
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
