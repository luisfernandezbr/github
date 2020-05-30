package internal

import (
	"time"

	"github.com/pinpt/go-common/datamodel"
	"github.com/pinpt/go-common/datetime"
	"github.com/pinpt/integration-sdk/sourcecode"
)

type reviews struct {
	TotalCount int
	PageInfo   pageInfo
	Nodes      []review
}

type review struct {
	ID        string    `json:"id"`
	State     string    `json:"state"`
	CreatedAt time.Time `json:"createdAt"`
	Author    author    `json:"author"`
	URL       string    `json:"url"`
}

func (r review) ToModel(customerID string, repoID string, prID string) datamodel.Model {
	prreview := &sourcecode.PullRequestReview{}
	prreview.CustomerID = customerID
	prreview.ID = sourcecode.NewPullRequestReviewID(customerID, r.ID, refType, repoID)
	prreview.RefID = r.ID
	prreview.RefType = refType
	prreview.RepoID = repoID
	prreview.PullRequestID = prID
	prreview.URL = r.URL
	cd, _ := datetime.NewDateWithTime(r.CreatedAt)
	prreview.CreatedDate = sourcecode.PullRequestReviewCreatedDate{
		Epoch:   cd.Epoch,
		Rfc3339: cd.Rfc3339,
		Offset:  cd.Offset,
	}
	switch r.State {
	case "PENDING":
		prreview.State = sourcecode.PullRequestReviewStatePending
	case "COMMENTED":
		prreview.State = sourcecode.PullRequestReviewStateCommented
	case "APPROVED":
		prreview.State = sourcecode.PullRequestReviewStateApproved
	case "CHANGES_REQUESTED":
		prreview.State = sourcecode.PullRequestReviewStateChangesRequested
	case "DISMISSED":
		prreview.State = sourcecode.PullRequestReviewStateDismissed
	}
	return prreview
}
