package internal

import (
	"fmt"
	"strconv"
	"time"

	"github.com/google/go-github/v32/github"
	"github.com/pinpt/agent/v4/sdk"
)

type milestoneCommon struct {
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Number      int        `json:"number"`
	URL         string     `json:"url"`
	HTMLUrl     string     `json:"html_url"`
	Closed      bool       `json:"closed"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	ClosedAt    *time.Time `json:"closedAt"`
	DueAt       *time.Time `json:"dueOn"`
	State       string     `json:"state"`
}

func (m milestoneCommon) ToModel(logger sdk.Logger, userManager *UserManager, customerID string, integrationInstanceID string, repoName string, projectID string) (*sdk.WorkIssue, error) {
	var issue sdk.WorkIssue
	issue.CustomerID = customerID
	issue.IntegrationInstanceID = sdk.StringPointer(integrationInstanceID)
	issue.RefType = refType
	issue.Identifier = fmt.Sprintf("%s#%d", m.Title, m.Number)
	issue.URL = m.HTMLUrl
	issue.Title = m.Title
	issue.Description = toHTML(m.Description)
	issue.ProjectIds = []string{projectID}
	issue.Active = true
	sdk.ConvertTimeToDateModel(m.CreatedAt, &issue.CreatedDate)
	sdk.ConvertTimeToDateModel(m.UpdatedAt, &issue.UpdatedDate)
	if m.DueAt != nil {
		sdk.ConvertTimeToDateModel(*m.DueAt, &issue.DueDate)
	}
	if m.Closed {
		issue.Status = "CLOSED"
	} else {
		issue.Status = "OPEN"
	}
	issue.Type = epicIssueTypeName
	issue.TypeID = sdk.NewWorkIssueTypeID(customerID, refType, epicIssueTypeRefID)
	issue.Transitions = make([]sdk.WorkIssueTransitions, 0)
	if m.Closed {
		issue.Transitions = append(issue.Transitions, sdk.WorkIssueTransitions{
			RefID: "open",
			Name:  "Open",
		})
	} else {
		issue.Transitions = append(issue.Transitions, sdk.WorkIssueTransitions{
			RefID: "close",
			Name:  "Close",
		})
	}
	return &issue, nil
}

type milestone struct {
	milestoneCommon
	ID      string `json:"id"`
	Creator author `json:"creator"`
}

type milestoneRest struct {
	milestoneCommon
	ID      int64   `json:"id"`
	Creator author2 `json:"creator"`
}

type milestoneNodes struct {
	PageInfo pageInfo    `json:"pageInfo"`
	Nodes    []milestone `json:"nodes"`
}

type repositoryMilestonesResult struct {
	RateLimit  rateLimit
	Repository struct {
		TotalCount int            `json:"totalCount"`
		Milestones milestoneNodes `json:"milestones"`
	} `json:"repository"`
}

func (g *GithubIntegration) fromMilestoneEvent(logger sdk.Logger, client sdk.GraphQLClient, userManager *UserManager, control sdk.Control, customerID string, integrationInstanceID string, event *github.MilestoneEvent) (*sdk.WorkIssue, error) {
	var milestone milestone
	m := event.Milestone
	milestone.ID = m.GetNodeID()
	milestone.Title = m.GetTitle()
	milestone.Description = m.GetDescription()
	milestone.Number = m.GetNumber()
	milestone.URL = m.GetHTMLURL()
	milestone.Closed = m.GetState() == "CLOSED"
	milestone.CreatedAt = m.GetCreatedAt()
	milestone.UpdatedAt = m.GetUpdatedAt()
	milestone.ClosedAt = m.ClosedAt
	milestone.DueAt = m.DueOn
	milestone.State = m.GetState()
	milestone.Creator = userToAuthor(m.Creator)
	projectID := sdk.NewWorkProjectID(customerID, event.Repo.GetNodeID(), refType)
	return milestone.ToModel(logger, userManager, customerID, integrationInstanceID, event.Repo.GetFullName(), projectID)
}

func (m milestone) ToModel(logger sdk.Logger, userManager *UserManager, customerID string, integrationInstanceID string, repoName string, projectID string) (*sdk.WorkIssue, error) {

	issue, err := m.milestoneCommon.ToModel(logger, userManager, customerID, integrationInstanceID, repoName, projectID)
	if err != nil {
		return nil, err
	}

	issue.RefID = m.ID
	issue.ID = sdk.NewWorkIssueID(customerID, issue.RefID, refType)
	issue.CreatorRefID = m.Creator.RefID(customerID)
	issue.ReporterRefID = m.Creator.RefID(customerID)
	if err := userManager.emitAuthor(logger, m.Creator); err != nil {
		return nil, err
	}
	return issue, nil
}

func (m milestoneRest) ToModel(logger sdk.Logger, userManager *UserManager, customerID string, integrationInstanceID string, repoName string, projectID string) (*sdk.WorkIssue, error) {

	issue, err := m.milestoneCommon.ToModel(logger, userManager, customerID, integrationInstanceID, repoName, projectID)
	if err != nil {
		return nil, err
	}

	issue.RefID = strconv.FormatInt(m.ID, 10)
	issue.ID = sdk.NewWorkIssueID(customerID, issue.RefID, refType)
	issue.CreatorRefID = m.Creator.RefID(customerID)
	issue.ReporterRefID = m.Creator.RefID(customerID)
	if err := userManager.emitAuthor2(logger, m.Creator); err != nil {
		return nil, err
	}
	return issue, nil

}

func createMilestone(logger sdk.Logger, client sdk.HTTPClient, userManager *UserManager, input map[string]interface{}, repo sdk.NameRefID) (*sdk.WorkIssue, error) {

	var res milestoneRest

	path := sdk.JoinURL("repos", *repo.Name, "milestones")

	input["description"] = input["body"]
	input["state"] = "open"
	delete(input, "body")
	delete(input, "repositoryID")

	opts := []sdk.WithHTTPOption{
		sdk.WithHTTPHeader("accept", "application/vnd.github.v3+json"),
		sdk.WithEndpoint(path),
	}

	response, err := client.Post(sdk.StringifyReader(input), &res, opts...)
	if err != nil {
		return nil, fmt.Errorf("error creating milestone, %s %s", string(response.Body), err)
	}

	return res.ToModel(logger, userManager, userManager.customerID, userManager.instanceid, *repo.Name, *repo.RefID)

}
