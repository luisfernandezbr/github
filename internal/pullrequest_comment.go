package internal

import (
	"time"

	"github.com/pinpt/agent.next/sdk"
)

type pullrequestcommentsNode struct {
	Cursor string
	Node   pullrequestcomment
}

type pullrequestcomments struct {
	TotalCount int
	PageInfo   pageInfo
	Edges      []pullrequestcommentsNode
}

type pullrequestcomment struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Author    author    `json:"author"`
	URL       string    `json:"url"`
	Body      string    `json:"bodyHTML"`
}

func (c pullrequestcomment) ToModel(userManager *UserManager, customerID string, repoID string, pullRequestID string) *sdk.SourceCodePullRequestComment {
	comment := &sdk.SourceCodePullRequestComment{}
	comment.CustomerID = customerID
	comment.RepoID = repoID
	comment.PullRequestID = pullRequestID
	comment.ID = sdk.NewSourceCodePullRequestCommentID(customerID, c.ID, refType, repoID)
	comment.RefID = c.ID
	comment.RefType = refType
	comment.Body = c.Body
	comment.URL = c.URL
	cd, _ := sdk.NewDateWithTime(c.CreatedAt)
	comment.CreatedDate = sdk.SourceCodePullRequestCommentCreatedDate{
		Epoch:   cd.Epoch,
		Rfc3339: cd.Rfc3339,
		Offset:  cd.Offset,
	}
	ud, _ := sdk.NewDateWithTime(c.UpdatedAt)
	comment.UpdatedDate = sdk.SourceCodePullRequestCommentUpdatedDate{
		Epoch:   ud.Epoch,
		Rfc3339: ud.Rfc3339,
		Offset:  ud.Offset,
	}
	comment.UserRefID = c.Author.RefID(customerID)
	userManager.emitAuthor(c.Author)
	return comment
}
