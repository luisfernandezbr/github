package internal

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v32/github"
	"github.com/pinpt/agent.next/sdk"
)

type pullrequestcommentsNode struct {
	Cursor string             `json:"cursor"`
	Node   pullrequestcomment `json:"node"`
}

type pullrequestcomments struct {
	TotalCount int                       `json:"totalCount"`
	PageInfo   pageInfo                  `json:"pageInfo"`
	Edges      []pullrequestcommentsNode `json:"edges"`
}

type pullrequestcomment struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Author    author    `json:"author"`
	URL       string    `json:"url"`
	Body      string    `json:"bodyHTML"`
}

func isIssueCommentPR(event *github.IssueCommentEvent) bool {
	return event.Issue.IsPullRequest()
}

func prCommentNumberToNodeID(id int64) string {
	return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("012:IssueComment%d", id)))
}

func pullRequestURLToNumber(url string) int64 {
	i := strings.LastIndex(url, "/")
	val := url[i+1:]
	num, _ := strconv.ParseInt(val, 10, 32)
	return num
}

func (g *GithubIntegration) fetchPullRequestNodeIDFromIssueID(client sdk.GraphQLClient, repoLogin, repoName string, id int64) (string, error) {
	variables := map[string]interface{}{
		"name":   repoName,
		"owner":  repoLogin,
		"number": id,
	}
	var res struct {
		Repository struct {
			PullRequest struct {
				ID string `json:"id"`
			} `json:"pullRequest"`
		} `json:"repository"`
	}
	if err := client.Query(pullrequestNodeIDQuery, variables, &res); err != nil {
		return "", err
	}
	return res.Repository.PullRequest.ID, nil
}

func (g *GithubIntegration) fromPullRequestCommentEvent(logger sdk.Logger, client sdk.GraphQLClient, userManager *UserManager, control sdk.Control, customerID string, commentEvent *github.IssueCommentEvent) (*sdk.SourceCodePullRequestComment, error) {
	var comment pullrequestcomment
	theComment := commentEvent.GetComment()
	comment.ID = prCommentNumberToNodeID(theComment.GetID())
	comment.CreatedAt = theComment.GetCreatedAt()
	comment.UpdatedAt = theComment.GetUpdatedAt()
	comment.Author = userToAuthor(theComment.GetUser())
	comment.URL = theComment.GetHTMLURL()
	// comment.Active = true // FIXME: add
	if theComment.Body != nil {
		comment.Body = toHTML(theComment.GetBody())
	}
	repoID := sdk.NewSourceCodeRepoID(customerID, commentEvent.Repo.GetNodeID(), refType)
	// unfortunately, we have to make a graphql query to convert the PR number to the PR nodeid
	prNum := pullRequestURLToNumber(commentEvent.Issue.PullRequestLinks.GetURL())
	prNodeID, err := g.fetchPullRequestNodeIDFromIssueID(client, commentEvent.GetRepo().GetOwner().GetLogin(), commentEvent.GetRepo().GetName(), prNum)
	if err != nil {
		return nil, fmt.Errorf("error fetching pull request node id: %w", err)
	}
	prID := sdk.NewSourceCodePullRequestID(customerID, prNodeID, refType, repoID)
	return comment.ToModel(logger, userManager, customerID, repoID, prID)
}

func (c pullrequestcomment) ToModel(logger sdk.Logger, userManager *UserManager, customerID string, repoID string, pullRequestID string) (*sdk.SourceCodePullRequestComment, error) {
	comment := &sdk.SourceCodePullRequestComment{}
	comment.CustomerID = customerID
	comment.RepoID = repoID
	comment.PullRequestID = pullRequestID
	comment.ID = sdk.NewSourceCodePullRequestCommentID(customerID, c.ID, refType, repoID)
	comment.RefID = c.ID
	comment.RefType = refType
	comment.Body = c.Body
	comment.URL = c.URL
	comment.IntegrationInstanceID = sdk.StringPointer(userManager.instanceid)
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
	if err := userManager.emitAuthor(logger, c.Author); err != nil {
		return nil, err
	}
	comment.UserRefID = c.Author.RefID(customerID)
	return comment, nil
}
