package internal

import (
	"fmt"
	"time"

	"github.com/pinpt/go-common/datetime"
	"github.com/pinpt/integration-sdk/sourcecode"
)

type pullrequest struct {
	ID          string              `json:"id"`
	Body        string              `json:"bodyHTML"`
	URL         string              `json:"url"`
	Closed      bool                `json:"closed"`
	Draft       bool                `json:"draft"`
	Locked      bool                `json:"locked"`
	Merged      bool                `json:"merged"`
	Number      int                 `json:"number"`
	State       string              `json:"state"`
	Title       string              `json:"title"`
	CreatedAt   time.Time           `json:"createdAt"`
	UpdatedAt   time.Time           `json:"updatedAt"`
	MergedAt    time.Time           `json:"mergedAt"`
	Author      author              `json:"author"`
	Branch      string              `json:"branch"`
	MergeCommit oidProp             `json:"mergeCommit"`
	MergedBy    author              `json:"mergedBy"`
	Commits     pullrequestcommits  `json:"commits"`
	Reviews     pullrequestreviews  `json:"reviews"`
	Comments    pullrequestcomments `json:"comments"`
}

func setPullRequestCommits(pullrequest *sourcecode.PullRequest, commits []*sourcecode.PullRequestCommit) {
	commitids := []string{}
	commitshas := []string{}
	// commits come from Github in the latest to earliest
	for i := len(commits) - 1; i >= 0; i-- {
		commit := commits[i]
		commitshas = append(commitshas, commit.Sha)
		commitids = append(commitids, sourcecode.NewCommitID(pullrequest.CustomerID, commit.Sha, refType, pullrequest.RepoID))
	}
	pullrequest.CommitShas = commitshas
	pullrequest.CommitIds = commitids
	if len(commitids) > 0 {
		pullrequest.BranchID = sourcecode.NewBranchID(refType, pullrequest.RepoID, pullrequest.CustomerID, pullrequest.BranchName, pullrequest.CommitIds[0])
	} else {
		pullrequest.BranchID = sourcecode.NewBranchID(refType, pullrequest.RepoID, pullrequest.CustomerID, pullrequest.BranchName, "")
	}
	for _, commit := range commits {
		commit.BranchID = pullrequest.BranchID
	}
}

func (pr pullrequest) ToModel(userManager *UserManager, customerID string, repoName string, repoID string) *sourcecode.PullRequest {
	// FIXME: implement the remaining fields
	pullrequest := &sourcecode.PullRequest{}
	pullrequest.ID = sourcecode.NewPullRequestID(customerID, pr.ID, refType, repoID)
	pullrequest.CustomerID = customerID
	pullrequest.RepoID = repoID
	pullrequest.RefID = pr.ID
	pullrequest.RefType = refType
	pullrequest.Title = pr.Title
	pullrequest.URL = pr.URL
	pullrequest.Description = pr.Body
	pullrequest.Draft = pr.Draft
	pullrequest.CreatedByRefID = pr.Author.RefID(customerID)
	userManager.emitAuthor(pr.Author)
	pullrequest.BranchName = pr.Branch
	pullrequest.Identifier = fmt.Sprintf("%s#%d", repoName, pr.Number)
	if pr.Merged {
		pullrequest.MergeSha = pr.MergeCommit.Oid
		pullrequest.MergeCommitID = sourcecode.NewCommitID(customerID, pr.MergeCommit.Oid, refType, repoID)
		md, _ := datetime.NewDateWithTime(pr.MergedAt)
		pullrequest.MergedDate = sourcecode.PullRequestMergedDate{
			Epoch:   md.Epoch,
			Rfc3339: md.Rfc3339,
			Offset:  md.Offset,
		}
		pullrequest.MergedByRefID = pr.MergedBy.RefID(customerID)
		userManager.emitAuthor(pr.MergedBy)
	}
	cd, _ := datetime.NewDateWithTime(pr.CreatedAt)
	pullrequest.CreatedDate = sourcecode.PullRequestCreatedDate{
		Epoch:   cd.Epoch,
		Rfc3339: cd.Rfc3339,
		Offset:  cd.Offset,
	}
	ud, _ := datetime.NewDateWithTime(pr.UpdatedAt)
	pullrequest.UpdatedDate = sourcecode.PullRequestUpdatedDate{
		Epoch:   ud.Epoch,
		Rfc3339: ud.Rfc3339,
		Offset:  ud.Offset,
	}
	switch pr.State {
	case "OPEN":
		if pr.Locked {
			pullrequest.Status = sourcecode.PullRequestStatusLocked
		} else {
			pullrequest.Status = sourcecode.PullRequestStatusOpen
		}
	case "CLOSED":
		pullrequest.Status = sourcecode.PullRequestStatusClosed
		pullrequest.ClosedByRefID = "" // TODO
		// userManager.emit(pr.Author)
		pullrequest.ClosedDate = sourcecode.PullRequestClosedDate{
			Epoch:   ud.Epoch,
			Rfc3339: ud.Rfc3339,
			Offset:  ud.Offset,
		}
	case "MERGED":
		pullrequest.Status = sourcecode.PullRequestStatusMerged
	}
	return pullrequest
}

type repositoryPullrequests struct {
	Repository repository `json:"repository"`
	RateLimit  rateLimit  `json:"rateLimit"`
}

type pullrequests struct {
	TotalCount int
	PageInfo   pageInfo
	Nodes      []pullrequest
}

type pullrequestPagedCommit struct {
	Commit pullrequestCommit `json:"commit"`
}

type pullrequestPagedCommitNode struct {
	Nodes []pullrequestPagedCommit `json:"nodes"`
}

type pullrequestPagedCommits struct {
	TotalCount int
	PageInfo   pageInfo
	Commits    pullrequestPagedCommitNode `json:"commits"`
}

type pullrequestPagedCommitsResult struct {
	RateLimit rateLimit               `json:"rateLimit"`
	Node      pullrequestPagedCommits `json:"node"`
}
