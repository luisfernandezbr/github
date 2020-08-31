package internal

import (
	"fmt"
	"time"

	"github.com/google/go-github/v32/github"
	"github.com/pinpt/agent.next/sdk"
)

type pullrequestreviewrequestsNode struct {
	Cursor string                   `json:"cursor"`
	Node   pullrequestreviewrequest `json:"node"`
}

type pullrequestreviewrequests struct {
	TotalCount int
	PageInfo   pageInfo
	Edges      []pullrequestreviewrequestsNode
}

type pullrequestreviewrequest struct {
	ID                string `json:"id"`
	RequestedReviewer author `json:"requestedReviewer"`
}

func (g *GithubIntegration) fromPullRequestReviewRequestedEvent(logger sdk.Logger, client sdk.GraphQLClient, userManager *UserManager, control sdk.Control, customerID string, prEvent *github.PullRequestEvent) (*sdk.SourceCodePullRequestReviewRequest, error) {
	repoID := sdk.NewSourceCodeRepoID(customerID, prEvent.Repo.GetNodeID(), refType)
	prID := sdk.NewSourceCodePullRequestID(customerID, prEvent.PullRequest.GetNodeID(), refType, repoID)
	reviewer := prEvent.GetRequestedReviewer()
	if reviewer == nil {
		return nil, fmt.Errorf("error no requested reviewer for pr review request on pr %s", prID)
	}
	var updatedAt time.Time
	if prEvent.PullRequest.UpdatedAt != nil {
		updatedAt = *prEvent.PullRequest.UpdatedAt
	}
	// TODO(robin): figure out ref_id
	reviewRequest := reviewRequest(customerID, userManager.instanceid, repoID, prID, *reviewer.NodeID, *prEvent.GetSender().NodeID, updatedAt)
	if *prEvent.Action == "review_request_removed" {
		reviewRequest.Active = false
	}
	if err := userManager.emitAuthor(logger, userToAuthor(reviewer)); err != nil {
		return nil, err
	}
	return &reviewRequest, nil
}

// reviewRequest sets everything
func reviewRequest(customerID string, integrationInstanceID string, repoID string, prID string, requestedReviewerID string, senderRefID string, updatedAt time.Time) sdk.SourceCodePullRequestReviewRequest {
	return sdk.SourceCodePullRequestReviewRequest{
		CustomerID:             customerID,
		ID:                     sdk.NewSourceCodePullRequestReviewRequestID(customerID, refType, prID, requestedReviewerID),
		RefType:                refType,
		RepoID:                 repoID,
		PullRequestID:          prID,
		Active:                 true,
		CreatedDate:            sdk.SourceCodePullRequestReviewRequestCreatedDate(*(sdk.NewDateWithTime(updatedAt))),
		IntegrationInstanceID:  sdk.StringPointer(integrationInstanceID),
		RequestedReviewerRefID: requestedReviewerID,
		SenderRefID:            senderRefID,
	}
}

func (r pullrequestreviewrequest) ToModel(logger sdk.Logger, userManager *UserManager, customerID string, repoID string, prID string, prUpdatedDate time.Time) (*sdk.SourceCodePullRequestReviewRequest, error) {
	reviewRequest := reviewRequest(customerID, userManager.instanceid, repoID, prID, r.RequestedReviewer.ID, "", time.Now())
	// TODO(robin): figure out ref_id
	if err := userManager.emitAuthor(logger, r.RequestedReviewer); err != nil {
		return nil, err
	}
	return &reviewRequest, nil
}
