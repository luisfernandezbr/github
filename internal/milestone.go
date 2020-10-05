package internal

import (
	"fmt"
	"time"

	"github.com/google/go-github/v32/github"
	"github.com/pinpt/agent/v4/sdk"
)

type milestone struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Number      int        `json:"number"`
	URL         string     `json:"url"`
	Closed      bool       `json:"closed"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	ClosedAt    *time.Time `json:"closedAt"`
	DueAt       *time.Time `json:"dueOn"`
	State       string     `json:"state"`
	Creator     author     `json:"creator"`
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
	var issue sdk.WorkIssue
	issue.CustomerID = customerID
	issue.IntegrationInstanceID = sdk.StringPointer(integrationInstanceID)
	issue.RefID = m.ID
	issue.RefType = refType
	issue.Identifier = fmt.Sprintf("%s#%d", m.Title, m.Number)
	issue.URL = m.URL
	issue.Title = m.Title
	issue.Description = toHTML(m.Description)
	issue.ProjectID = sdk.StringPointer(projectID)
	issue.Active = true
	issue.ID = sdk.NewWorkIssueID(customerID, m.ID, refType)
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
	issue.CreatorRefID = m.Creator.RefID(customerID)
	issue.ReporterRefID = m.Creator.RefID(customerID)
	if err := userManager.emitAuthor(logger, m.Creator); err != nil {
		return nil, err
	}
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
