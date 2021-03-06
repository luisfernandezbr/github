package internal

import (
	"time"

	"github.com/pinpt/agent/v4/sdk"
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

func (c pullrequestCommit) ToModel(logger sdk.Logger, userManager *UserManager, customerID string, repoID string, prID string) (*sdk.SourceCodePullRequestCommit, error) {
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
	commit.Active = true
	dt := sdk.NewDateWithTime(c.Date)
	commit.IntegrationInstanceID = sdk.StringPointer(userManager.instanceid)
	commit.CreatedDate = sdk.SourceCodePullRequestCommitCreatedDate{
		Epoch:   dt.Epoch,
		Rfc3339: dt.Rfc3339,
		Offset:  dt.Offset,
	}
	if err := userManager.emitGitUser(logger, c.Author); err != nil {
		return nil, err
	}
	if err := userManager.emitGitUser(logger, c.Committer); err != nil {
		return nil, err
	}
	return commit, nil
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
