package internal

import (
	"time"

	"github.com/pinpt/go-common/datetime"
	"github.com/pinpt/integration-sdk/sourcecode"
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

func (c pullrequestCommit) ToModel(customerID string, repoID string, branchID string) *sourcecode.PullRequestCommit {
	commit := &sourcecode.PullRequestCommit{}
	commit.CustomerID = customerID
	commit.RepoID = repoID
	commit.ID = sourcecode.NewCommitID(customerID, c.Sha, refType, repoID)
	commit.Sha = c.Sha
	commit.Message = c.Message
	commit.Additions = c.Additions
	commit.Deletions = c.Deletions
	commit.RefType = refType
	commit.RefID = c.Sha
	commit.URL = c.URL
	commit.BranchID = branchID
	commit.AuthorRefID = c.Author.RefID()
	commit.CommitterRefID = c.Committer.RefID()
	dt, _ := datetime.NewDateWithTime(c.Date)
	commit.CreatedDate = sourcecode.PullRequestCommitCreatedDate{
		Epoch:   dt.Epoch,
		Rfc3339: dt.Rfc3339,
		Offset:  dt.Offset,
	}
	return commit
}

type pullrequestcommit struct {
	Commit pullrequestCommit `json:"commit"`
}

type pullrequestcommits struct {
	TotalCount int
	PageInfo   pageInfo
	Nodes      []pullrequestcommit
}
