package internal

import (
	"fmt"
	"time"

	"github.com/google/go-github/v32/github"
	"github.com/pinpt/agent.next/sdk"
)

type label struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type labelNode struct {
	Nodes []label `json:"nodes"`
}

type assigneesNode struct {
	Nodes []author `json:"nodes"`
}

type comment struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Author    author    `json:"author"`
}

type commentsNode struct {
	Nodes []comment `json:"nodes"`
}

type issueMilestone struct {
	ID string `json:"id"`
}

type timelineItem struct {
	ID        string    `json:"id"`
	Type      string    `json:"__typename"`
	CreatedAt time.Time `json:"createdAt"`
	Actor     author    `json:"actor"`
	// these are optional based on the event
	Assignee *author `json:"assignee,omitempty"`
	Label    *label  `json:"label,omitempty"`
	Title    *string `json:"currentTitle,omitempty"`
}

type timelineItems struct {
	Nodes []timelineItem `json:"nodes"`
}

type issue struct {
	ID        string          `json:"id"`
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
	ClosedAt  *time.Time      `json:"closedAt"`
	State     string          `json:"state"`
	URL       string          `json:"url"`
	Title     string          `json:"title"`
	Body      string          `json:"body"`
	Closed    bool            `json:"closed"`
	Labels    labelNode       `json:"labels"`
	Comments  commentsNode    `json:"comments"`
	Assignees assigneesNode   `json:"assignees"`
	Author    author          `json:"author"`
	Number    int             `json:"number"`
	Milestone *issueMilestone `json:"milestone"`
	Timeline  *timelineItems  `json:"timelineItems"`
}

type issueNode struct {
	TotalCount int      `json:"totalCount"`
	PageInfo   pageInfo `json:"pageInfo"`
	Nodes      []issue  `json:"nodes"`
}

type issueRepository struct {
	Issues issueNode `json:"issues"`
}

type issueResult struct {
	RateLimit  rateLimit       `json:"rateLimit"`
	Repository issueRepository `json:"repository"`
}

const (
	issueTypeCacheKeyPrefix = "issue_type_"
	defaultIssueTypeRefID   = ""
	defaultIssueTypeName    = "Task"
	epicIssueTypeRefID      = "epic"
	epicIssueTypeName       = "Epic"
)

func (g *GithubIntegration) processDefaultIssueType(logger sdk.Logger, pipe sdk.Pipe, state sdk.State, customerID string, integrationInstanceID string, historical bool) error {
	key := issueTypeCacheKeyPrefix
	if historical || !state.Exists(key) {
		var t sdk.WorkIssueType
		t.CustomerID = customerID
		t.IntegrationInstanceID = sdk.StringPointer(integrationInstanceID)
		t.RefID = defaultIssueTypeRefID
		t.RefType = refType
		t.Name = defaultIssueTypeName
		t.Description = sdk.StringPointer("default issue type")
		t.MappedType = sdk.WorkIssueTypeMappedTypeTask
		t.ID = sdk.NewWorkIssueTypeID(customerID, refType, t.RefID)
		if err := pipe.Write(&t); err != nil {
			return err
		}
		sdk.LogDebug(logger, "writing a default issue type to state")
		if err := state.Set(key, t.ID); err != nil {
			return err
		}
		t.RefID = epicIssueTypeRefID
		t.RefType = refType
		t.Name = epicIssueTypeName
		t.Description = sdk.StringPointer("milestone issue type")
		t.MappedType = sdk.WorkIssueTypeMappedTypeEpic
		t.ID = sdk.NewWorkIssueTypeID(customerID, refType, t.RefID)
		if err := pipe.Write(&t); err != nil {
			return err
		}
		sdk.LogDebug(logger, "writing a milestone issue type to state")
		if err := state.Set(key, t.ID); err != nil {
			return err
		}
	}
	return nil
}

func (l label) ToModel(logger sdk.Logger, state sdk.State, customerID string, integrationInstanceID string, historical bool) (*sdk.WorkIssueType, error) {
	key := issueTypeCacheKeyPrefix + l.ID
	if historical || !state.Exists(key) {
		switch l.Name {
		case "bug":
			var t sdk.WorkIssueType
			t.CustomerID = customerID
			t.IntegrationInstanceID = sdk.StringPointer(integrationInstanceID)
			t.RefID = l.ID
			t.RefType = refType
			t.Name = "Bug"
			t.Description = sdk.StringPointer(l.Description)
			t.MappedType = sdk.WorkIssueTypeMappedTypeBug
			t.ID = sdk.NewWorkIssueTypeID(customerID, refType, l.ID)
			err := state.Set(key, t.ID)
			sdk.LogDebug(logger, "creating issue type", "name", t.Name, "id", t.RefID, "err", err)
			return &t, err
		case "enhancement":
			var t sdk.WorkIssueType
			t.CustomerID = customerID
			t.IntegrationInstanceID = sdk.StringPointer(integrationInstanceID)
			t.RefID = l.ID
			t.RefType = refType
			t.Name = "Enhancement"
			t.MappedType = sdk.WorkIssueTypeMappedTypeEnhancement
			t.Description = sdk.StringPointer(l.Description)
			t.ID = sdk.NewWorkIssueTypeID(customerID, refType, l.ID)
			err := state.Set(key, t.ID)
			sdk.LogDebug(logger, "creating issue type", "name", t.Name, "id", t.RefID, "err", err)
			return &t, err
		}
	}
	return nil, nil
}

func setIssueType(issue *sdk.WorkIssue, labels []label) {
	for _, label := range labels {
		switch label.Name {
		case "bug":
			issue.Type = "Bug"
			issue.TypeID = label.ID
			return
		case "enhancement":
			issue.Type = "Enhancement"
			issue.TypeID = label.ID
			return
		}
	}
	issue.Type = defaultIssueTypeName // when no label, default to Task?
	issue.TypeID = sdk.NewWorkIssueTypeID(issue.CustomerID, refType, defaultIssueTypeRefID)
}

func (g *GithubIntegration) fromIssueEvent(logger sdk.Logger, userManager *UserManager, integrationInstanceID string, customerID string, event *github.IssuesEvent) (*sdk.WorkIssue, error) {
	var issue issue
	theIssue := event.Issue
	issue.ID = theIssue.GetNodeID()
	issue.CreatedAt = theIssue.GetCreatedAt()
	issue.UpdatedAt = theIssue.GetUpdatedAt()
	issue.ClosedAt = theIssue.ClosedAt
	issue.State = theIssue.GetState()
	issue.URL = theIssue.GetHTMLURL()
	issue.Title = theIssue.GetTitle()
	issue.Body = theIssue.GetBody()
	issue.Closed = theIssue.GetState() == "CLOSED"
	issue.Number = theIssue.GetNumber()
	issue.Author = userToAuthor(theIssue.User)
	issue.Assignees = assigneesNode{Nodes: make([]author, 0)}
	if theIssue.Milestone != nil {
		issue.Milestone = &issueMilestone{theIssue.Milestone.GetNodeID()}
	}
	for _, a := range theIssue.Assignees {
		issue.Assignees.Nodes = append(issue.Assignees.Nodes, userToAuthor(a))
	}
	issue.Labels = labelNode{Nodes: make([]label, 0)}
	for _, l := range theIssue.Labels {
		issue.Labels.Nodes = append(issue.Labels.Nodes, label{ID: l.GetNodeID(), Name: l.GetName()})
	}
	projectID := sdk.NewWorkProjectID(customerID, event.Repo.GetNodeID(), refType)
	return issue.ToModel(logger, userManager, customerID, integrationInstanceID, event.Repo.GetFullName(), projectID)
}

func (i issue) ToModel(logger sdk.Logger, userManager *UserManager, customerID string, integrationInstanceID string, repoName, projectID string) (*sdk.WorkIssue, error) {
	var issue sdk.WorkIssue
	issue.CustomerID = customerID
	issue.IntegrationInstanceID = sdk.StringPointer(integrationInstanceID)
	issue.RefID = i.ID
	issue.RefType = refType
	issue.Identifier = fmt.Sprintf("%s#%d", repoName, i.Number)
	issue.URL = i.URL
	issue.Title = i.Title
	issue.Description = toHTML(i.Body)
	issue.ProjectID = projectID
	issue.Active = true
	issue.ID = sdk.NewWorkIssueID(customerID, i.ID, refType)
	fmt.Println(issue.ID)
	if len(i.Labels.Nodes) > 0 {
		issue.Tags = make([]string, 0)
		for _, l := range i.Labels.Nodes {
			issue.Tags = append(issue.Tags, l.Name)
		}
	}
	sdk.ConvertTimeToDateModel(i.CreatedAt, &issue.CreatedDate)
	sdk.ConvertTimeToDateModel(i.UpdatedAt, &issue.UpdatedDate)
	if i.Closed {
		issue.Status = "CLOSED"
	} else {
		issue.Status = "OPEN"
	}
	setIssueType(&issue, i.Labels.Nodes)
	issue.CreatorRefID = i.Author.RefID(customerID)
	issue.ReporterRefID = i.Author.RefID(customerID)
	if err := userManager.emitAuthor(logger, i.Author); err != nil {
		return nil, err
	}
	if len(i.Assignees.Nodes) > 0 {
		issue.AssigneeRefID = i.Assignees.Nodes[0].RefID(customerID)
		if err := userManager.emitAuthor(logger, i.Assignees.Nodes[0]); err != nil {
			return nil, err
		}
	}
	if i.Milestone != nil {
		issue.ParentID = sdk.NewWorkIssueID(customerID, i.Milestone.ID, refType)
	}
	if i.Timeline != nil {
		// the changelog should be in the order from OLDEST to NEWEST
		issue.ChangeLog = make([]sdk.WorkIssueChangeLog, 0)
		var previousTitle, previousAssignee string
		for ordinal, node := range i.Timeline.Nodes {
			var changelog sdk.WorkIssueChangeLog
			sdk.ConvertTimeToDateModel(node.CreatedAt, &changelog.CreatedDate)
			changelog.Ordinal = int64(ordinal)
			changelog.UserID = node.Actor.RefID(customerID)
			changelog.RefID = node.ID
			if err := userManager.emitAuthor(logger, node.Actor); err != nil {
				return nil, err
			}
			switch node.Type {
			case "AssignedEvent":
				changelog.Field = sdk.WorkIssueChangeLogFieldAssigneeRefID
				changelog.From = previousAssignee
				changelog.To = node.Assignee.RefID(customerID)
				changelog.ToString = node.Assignee.Name
				previousAssignee = changelog.To
				if err := userManager.emitAuthor(logger, *node.Assignee); err != nil {
					return nil, err
				}
			case "UnassignedEvent":
				changelog.Field = sdk.WorkIssueChangeLogFieldAssigneeRefID
				changelog.From = previousAssignee
				previousAssignee = ""
			case "LabeledEvent":
			case "UnlabeledEvent":
			case "MilestonedEvent":
				changelog.Field = sdk.WorkIssueChangeLogFieldEpicID
			case "DemilestonedEvent":
				changelog.Field = sdk.WorkIssueChangeLogFieldEpicID
			case "RenamedTitleEvent":
				changelog.Field = sdk.WorkIssueChangeLogFieldTitle
				if node.Title != nil {
					changelog.From = previousTitle
					changelog.To = *node.Title
					previousTitle = *node.Title
				} else {
					previousTitle = ""
				}
			case "AddedToProjectEvent":
				changelog.Field = sdk.WorkIssueChangeLogFieldProjectID
			case "RemovedFromProjectEvent":
				changelog.Field = sdk.WorkIssueChangeLogFieldProjectID
			case "ClosedEvent":
				continue // currently no-op
			case "ReopenedEvent":
				continue // currently no-op
			}
			issue.ChangeLog = append(issue.ChangeLog, changelog)
		}
	}
	return &issue, nil
}

// TODO: linked_issues for PRs which are linked to an issue

func (g *GithubIntegration) fromIssueCommentEvent(logger sdk.Logger, client sdk.GraphQLClient, userManager *UserManager, control sdk.Control, customerID string, integrationInstanceID string, commentEvent *github.IssueCommentEvent) (*sdk.WorkIssueComment, error) {
	var comment comment
	theComment := commentEvent.GetComment()
	comment.ID = theComment.GetNodeID()
	comment.CreatedAt = theComment.GetCreatedAt()
	comment.UpdatedAt = theComment.GetUpdatedAt()
	comment.Author = userToAuthor(theComment.GetUser())
	comment.URL = theComment.GetHTMLURL()
	if theComment.Body != nil {
		comment.Body = toHTML(theComment.GetBody())
	}
	projectID := sdk.NewWorkProjectID(customerID, commentEvent.Repo.GetNodeID(), refType)
	issueID := sdk.NewWorkIssueID(customerID, commentEvent.Issue.GetNodeID(), refType)
	return comment.ToModel(logger, userManager, customerID, integrationInstanceID, projectID, issueID)
}

func (c comment) ToModel(logger sdk.Logger, userManager *UserManager, customerID string, integrationInstanceID string, projectID string, issueID string) (*sdk.WorkIssueComment, error) {
	var comment sdk.WorkIssueComment
	comment.CustomerID = customerID
	comment.RefID = c.ID
	comment.RefType = refType
	comment.IntegrationInstanceID = sdk.StringPointer(integrationInstanceID)
	comment.Body = toHTML(c.Body)
	comment.IssueID = issueID
	comment.ProjectID = projectID
	comment.URL = c.URL
	sdk.ConvertTimeToDateModel(c.CreatedAt, &comment.CreatedDate)
	sdk.ConvertTimeToDateModel(c.UpdatedAt, &comment.UpdatedDate)
	comment.ID = sdk.NewWorkIssueCommentID(customerID, c.ID, refType, projectID)
	comment.UserRefID = c.Author.RefID(customerID)
	err := userManager.emitAuthor(logger, c.Author)
	return &comment, err
}
