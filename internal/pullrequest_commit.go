package internal

import (
	"time"

	"github.com/pinpt/agent.next/sdk"
)

type pullrequestCommit struct {
	Sha       string    `json:"sha"`
	Message   string    `json:"message"`
	Date      time.Time `json:"authoredDate"`
	Additions int64     `json:"additions"`
	Deletions int64     `json:"deletions"`
	URL       string    `json:"url"`
	Author    gitUser   `json:"author"`
	Committer gitUser   `json:"committer"`
}

func (c pullrequestCommit) ToModel(userManager *UserManager, customerID string, repoID string, prID string) *sdk.SourceCodePullRequestCommit {
	commit := &sdk.SourceCodePullRequestCommit{}
	commit.CustomerID = customerID
	commit.RepoID = repoID
	commit.PullRequestID = prID
	commit.ID = sdk.NewSourceCodeCommitID(customerID, c.Sha, refType, repoID)
	commit.Sha = c.Sha
	commit.Message = c.Message
	commit.Additions = c.Additions
	commit.Deletions = c.Deletions
	commit.RefType = refType
	commit.RefID = c.Sha
	commit.URL = c.URL
	commit.AuthorRefID = c.Author.RefID(customerID)
	commit.CommitterRefID = c.Committer.RefID(customerID)
	dt, _ := sdk.NewDateWithTime(c.Date)
	commit.CreatedDate = sdk.SourceCodePullRequestCommitCreatedDate{
		Epoch:   dt.Epoch,
		Rfc3339: dt.Rfc3339,
		Offset:  dt.Offset,
	}
	userManager.emitGitUser(c.Author)
	userManager.emitGitUser(c.Committer)
	return commit
}

type pullrequestcommit struct {
	Commit pullrequestCommit `json:"commit"`
}

type pullrequestcommitNode struct {
	Cursor string
	Node   pullrequestcommit
}

type pullrequestcommits struct {
	TotalCount int
	PageInfo   pageInfo
	Edges      []pullrequestcommitNode
}
