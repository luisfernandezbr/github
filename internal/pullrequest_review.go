package internal

import (
	"time"

	"github.com/pinpt/agent.next/sdk"
)

type pullrequestreviewsNode struct {
	Cursor string
	Node   pullrequestreview
}

type pullrequestreviews struct {
	TotalCount int
	PageInfo   pageInfo
	Edges      []pullrequestreviewsNode
}

type pullrequestreview struct {
	ID        string    `json:"id"`
	State     string    `json:"state"`
	CreatedAt time.Time `json:"createdAt"`
	Author    author    `json:"author"`
	URL       string    `json:"url"`
}

func (r pullrequestreview) ToModel(userManager *UserManager, customerID string, repoID string, prID string) *sdk.SourceCodePullRequestReview {
	prreview := &sdk.SourceCodePullRequestReview{}
	prreview.CustomerID = customerID
	prreview.ID = sdk.NewSourceCodePullRequestReviewID(customerID, r.ID, refType, repoID)
	prreview.RefID = r.ID
	prreview.RefType = refType
	prreview.RepoID = repoID
	prreview.PullRequestID = prID
	prreview.URL = r.URL
	cd, _ := sdk.NewDateWithTime(r.CreatedAt)
	prreview.CreatedDate = sdk.SourceCodePullRequestReviewCreatedDate{
		Epoch:   cd.Epoch,
		Rfc3339: cd.Rfc3339,
		Offset:  cd.Offset,
	}
	switch r.State {
	case "PENDING":
		prreview.State = sdk.SourceCodePullRequestReviewStatePending
	case "COMMENTED":
		prreview.State = sdk.SourceCodePullRequestReviewStateCommented
	case "APPROVED":
		prreview.State = sdk.SourceCodePullRequestReviewStateApproved
	case "CHANGES_REQUESTED":
		prreview.State = sdk.SourceCodePullRequestReviewStateChangesRequested
	case "DISMISSED":
		prreview.State = sdk.SourceCodePullRequestReviewStateDismissed
	}
	prreview.UserRefID = r.Author.RefID(customerID)
	userManager.emitAuthor(r.Author)
	return prreview
}
