package internal

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-github/v32/github"
	"github.com/pinpt/agent/v4/sdk"
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
	Actor author `json:"actor"`
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
		t.IconURL = sdk.StringPointer("data:image/svg+xml,%3Csvg aria-hidden='true' focusable='false' data-prefix='fas' data-icon='tasks' class='svg-inline--fa fa-tasks fa-w-16' role='img' xmlns='http://www.w3.org/2000/svg' viewBox='0 0 512 512'%3E%3Cpath fill='currentColor' d='M139.61 35.5a12 12 0 0 0-17 0L58.93 98.81l-22.7-22.12a12 12 0 0 0-17 0L3.53 92.41a12 12 0 0 0 0 17l47.59 47.4a12.78 12.78 0 0 0 17.61 0l15.59-15.62L156.52 69a12.09 12.09 0 0 0 .09-17zm0 159.19a12 12 0 0 0-17 0l-63.68 63.72-22.7-22.1a12 12 0 0 0-17 0L3.53 252a12 12 0 0 0 0 17L51 316.5a12.77 12.77 0 0 0 17.6 0l15.7-15.69 72.2-72.22a12 12 0 0 0 .09-16.9zM64 368c-26.49 0-48.59 21.5-48.59 48S37.53 464 64 464a48 48 0 0 0 0-96zm432 16H208a16 16 0 0 0-16 16v32a16 16 0 0 0 16 16h288a16 16 0 0 0 16-16v-32a16 16 0 0 0-16-16zm0-320H208a16 16 0 0 0-16 16v32a16 16 0 0 0 16 16h288a16 16 0 0 0 16-16V80a16 16 0 0 0-16-16zm0 160H208a16 16 0 0 0-16 16v32a16 16 0 0 0 16 16h288a16 16 0 0 0 16-16v-32a16 16 0 0 0-16-16z'%3E%3C/path%3E%3C/svg%3E")
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
		t.IconURL = sdk.StringPointer("data:image/svg+xml,%3Csvg aria-hidden='true' focusable='false' data-prefix='fas' data-icon='cogs' class='svg-inline--fa fa-cogs fa-w-20' role='img' xmlns='http://www.w3.org/2000/svg' viewBox='0 0 640 512'%3E%3Cpath fill='currentColor' d='M512.1 191l-8.2 14.3c-3 5.3-9.4 7.5-15.1 5.4-11.8-4.4-22.6-10.7-32.1-18.6-4.6-3.8-5.8-10.5-2.8-15.7l8.2-14.3c-6.9-8-12.3-17.3-15.9-27.4h-16.5c-6 0-11.2-4.3-12.2-10.3-2-12-2.1-24.6 0-37.1 1-6 6.2-10.4 12.2-10.4h16.5c3.6-10.1 9-19.4 15.9-27.4l-8.2-14.3c-3-5.2-1.9-11.9 2.8-15.7 9.5-7.9 20.4-14.2 32.1-18.6 5.7-2.1 12.1.1 15.1 5.4l8.2 14.3c10.5-1.9 21.2-1.9 31.7 0L552 6.3c3-5.3 9.4-7.5 15.1-5.4 11.8 4.4 22.6 10.7 32.1 18.6 4.6 3.8 5.8 10.5 2.8 15.7l-8.2 14.3c6.9 8 12.3 17.3 15.9 27.4h16.5c6 0 11.2 4.3 12.2 10.3 2 12 2.1 24.6 0 37.1-1 6-6.2 10.4-12.2 10.4h-16.5c-3.6 10.1-9 19.4-15.9 27.4l8.2 14.3c3 5.2 1.9 11.9-2.8 15.7-9.5 7.9-20.4 14.2-32.1 18.6-5.7 2.1-12.1-.1-15.1-5.4l-8.2-14.3c-10.4 1.9-21.2 1.9-31.7 0zm-10.5-58.8c38.5 29.6 82.4-14.3 52.8-52.8-38.5-29.7-82.4 14.3-52.8 52.8zM386.3 286.1l33.7 16.8c10.1 5.8 14.5 18.1 10.5 29.1-8.9 24.2-26.4 46.4-42.6 65.8-7.4 8.9-20.2 11.1-30.3 5.3l-29.1-16.8c-16 13.7-34.6 24.6-54.9 31.7v33.6c0 11.6-8.3 21.6-19.7 23.6-24.6 4.2-50.4 4.4-75.9 0-11.5-2-20-11.9-20-23.6V418c-20.3-7.2-38.9-18-54.9-31.7L74 403c-10 5.8-22.9 3.6-30.3-5.3-16.2-19.4-33.3-41.6-42.2-65.7-4-10.9.4-23.2 10.5-29.1l33.3-16.8c-3.9-20.9-3.9-42.4 0-63.4L12 205.8c-10.1-5.8-14.6-18.1-10.5-29 8.9-24.2 26-46.4 42.2-65.8 7.4-8.9 20.2-11.1 30.3-5.3l29.1 16.8c16-13.7 34.6-24.6 54.9-31.7V57.1c0-11.5 8.2-21.5 19.6-23.5 24.6-4.2 50.5-4.4 76-.1 11.5 2 20 11.9 20 23.6v33.6c20.3 7.2 38.9 18 54.9 31.7l29.1-16.8c10-5.8 22.9-3.6 30.3 5.3 16.2 19.4 33.2 41.6 42.1 65.8 4 10.9.1 23.2-10 29.1l-33.7 16.8c3.9 21 3.9 42.5 0 63.5zm-117.6 21.1c59.2-77-28.7-164.9-105.7-105.7-59.2 77 28.7 164.9 105.7 105.7zm243.4 182.7l-8.2 14.3c-3 5.3-9.4 7.5-15.1 5.4-11.8-4.4-22.6-10.7-32.1-18.6-4.6-3.8-5.8-10.5-2.8-15.7l8.2-14.3c-6.9-8-12.3-17.3-15.9-27.4h-16.5c-6 0-11.2-4.3-12.2-10.3-2-12-2.1-24.6 0-37.1 1-6 6.2-10.4 12.2-10.4h16.5c3.6-10.1 9-19.4 15.9-27.4l-8.2-14.3c-3-5.2-1.9-11.9 2.8-15.7 9.5-7.9 20.4-14.2 32.1-18.6 5.7-2.1 12.1.1 15.1 5.4l8.2 14.3c10.5-1.9 21.2-1.9 31.7 0l8.2-14.3c3-5.3 9.4-7.5 15.1-5.4 11.8 4.4 22.6 10.7 32.1 18.6 4.6 3.8 5.8 10.5 2.8 15.7l-8.2 14.3c6.9 8 12.3 17.3 15.9 27.4h16.5c6 0 11.2 4.3 12.2 10.3 2 12 2.1 24.6 0 37.1-1 6-6.2 10.4-12.2 10.4h-16.5c-3.6 10.1-9 19.4-15.9 27.4l8.2 14.3c3 5.2 1.9 11.9-2.8 15.7-9.5 7.9-20.4 14.2-32.1 18.6-5.7 2.1-12.1-.1-15.1-5.4l-8.2-14.3c-10.4 1.9-21.2 1.9-31.7 0zM501.6 431c38.5 29.6 82.4-14.3 52.8-52.8-38.5-29.6-82.4 14.3-52.8 52.8z'%3E%3C/path%3E%3C/svg%3E")
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
			t.IconURL = sdk.StringPointer("data:image/svg+xml,%3Csvg aria-hidden='true' focusable='false' data-prefix='fas' data-icon='bug' class='svg-inline--fa fa-bug fa-w-16' role='img' xmlns='http://www.w3.org/2000/svg' viewBox='0 0 512 512'%3E%3Cpath fill='currentColor' d='M511.988 288.9c-.478 17.43-15.217 31.1-32.653 31.1H424v16c0 21.864-4.882 42.584-13.6 61.145l60.228 60.228c12.496 12.497 12.496 32.758 0 45.255-12.498 12.497-32.759 12.496-45.256 0l-54.736-54.736C345.886 467.965 314.351 480 280 480V236c0-6.627-5.373-12-12-12h-24c-6.627 0-12 5.373-12 12v244c-34.351 0-65.886-12.035-90.636-32.108l-54.736 54.736c-12.498 12.497-32.759 12.496-45.256 0-12.496-12.497-12.496-32.758 0-45.255l60.228-60.228C92.882 378.584 88 357.864 88 336v-16H32.666C15.23 320 .491 306.33.013 288.9-.484 270.816 14.028 256 32 256h56v-58.745l-46.628-46.628c-12.496-12.497-12.496-32.758 0-45.255 12.498-12.497 32.758-12.497 45.256 0L141.255 160h229.489l54.627-54.627c12.498-12.497 32.758-12.497 45.256 0 12.496 12.497 12.496 32.758 0 45.255L424 197.255V256h56c17.972 0 32.484 14.816 31.988 32.9zM257 0c-61.856 0-112 50.144-112 112h224C369 50.144 318.856 0 257 0z'%3E%3C/path%3E%3C/svg%3E")
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
			t.IconURL = sdk.StringPointer("data:image/svg+xml,%3Csvg aria-hidden='true' focusable='false' data-prefix='fas' data-icon='exclamation-circle' class='svg-inline--fa fa-exclamation-circle fa-w-16' role='img' xmlns='http://www.w3.org/2000/svg' viewBox='0 0 512 512'%3E%3Cpath fill='currentColor' d='M504 256c0 136.997-111.043 248-248 248S8 392.997 8 256C8 119.083 119.043 8 256 8s248 111.083 248 248zm-248 50c-25.405 0-46 20.595-46 46s20.595 46 46 46 46-20.595 46-46-20.595-46-46-46zm-43.673-165.346l7.418 136c.347 6.364 5.609 11.346 11.982 11.346h48.546c6.373 0 11.635-4.982 11.982-11.346l7.418-136c.375-6.874-5.098-12.654-11.982-12.654h-63.383c-6.884 0-12.356 5.78-11.981 12.654z'%3E%3C/path%3E%3C/svg%3E")
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
	issue.ProjectIds = []string{projectID}
	issue.Active = true
	issue.ID = sdk.NewWorkIssueID(customerID, i.ID, refType)
	if len(i.Labels.Nodes) > 0 {
		issue.Tags = make([]string, 0)
		for _, l := range i.Labels.Nodes {
			issue.Tags = append(issue.Tags, l.Name)
		}
	}
	sdk.ConvertTimeToDateModel(i.CreatedAt, &issue.CreatedDate)
	sdk.ConvertTimeToDateModel(i.UpdatedAt, &issue.UpdatedDate)
	if i.Closed {
		issue.Status = "Closed"
	} else {
		issue.Status = "Open"
	}
	issue.StatusID = sdk.NewWorkIssueStatusID(customerID, refType, issue.Status)
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
	issue.Transitions = make([]sdk.WorkIssueTransitions, 0)
	if i.Closed {
		issue.Transitions = append(issue.Transitions, sdk.WorkIssueTransitions{
			RefID: "open",
			Name:  "Open",
		})
	} else {
		issue.Transitions = append(issue.Transitions, sdk.WorkIssueTransitions{
			RefID: "close",
			Name:  "Closed",
		})
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
	comment.Active = true
	comment.RefID = c.ID
	comment.RefType = refType
	comment.IntegrationInstanceID = sdk.StringPointer(integrationInstanceID)
	comment.Body = toHTML(c.Body)
	comment.IssueID = issueID
	comment.ProjectID = projectID
	comment.URL = c.URL
	sdk.ConvertTimeToDateModel(c.CreatedAt, &comment.CreatedDate)
	sdk.ConvertTimeToDateModel(c.UpdatedAt, &comment.UpdatedDate)
	comment.ID = sdk.NewWorkIssueCommentID(customerID, c.ID, refType)
	comment.UserRefID = c.Author.RefID(customerID)
	err := userManager.emitAuthor(logger, c.Author)
	return &comment, err
}

const createIssueQuery = `mutation createIssue($repositoryID: ID!,$title: String!,$body: String!) {
	createIssue(input:{
	  repositoryId:$repositoryID,
	  title:$title,
	  body:$body
	}) {
	  issue{
		id
		title
		number
		url
		state
		repository {
			id
			nameWithOwner
		}
		createdAt
		updatedAt
		author {
			login
		}
		milestone {
			id
		}
	  }
	}
  }`

const createIssueQueryWithLabels = `mutation createIssue($repositoryID: ID!, $title: String!, $body: String!, $label: ID!) {
	createIssue(input:{
	  repositoryId:$repositoryID,
	  title:$title,
	  body:$body,
	  labelIds:[$label]
	}) {
	  issue{
		id
		title
		number
		url
		state
		repository {
			id
			nameWithOwner
		}
		createdAt
		updatedAt
		author {
			login
		}
		milestone {
			id
		}
	  }
	}
  }`

func (g *GithubIntegration) createIssue(logger sdk.Logger, userManager *UserManager, mutation *sdk.WorkIssueCreateMutation, user sdk.MutationUser) (*sdk.MutationResponse, error) {

	var c sdk.Config
	c.APIKeyAuth = user.APIKeyAuth
	c.BasicAuth = user.BasicAuth
	c.OAuth2Auth = user.OAuth2Auth
	_, client, err := g.newGraphClient(logger, c)
	if err != nil {
		return nil, fmt.Errorf("error creating http client: %w", err)
	}
	_, httpClient, err := g.newHTTPClient(logger, c)
	if err != nil {
		return nil, fmt.Errorf("error creating http client: %w", err)
	}

	input, issueType, err := makeCreateMutation(logger, *mutation.Project.RefID, mutation.Fields)
	if err != nil {
		return nil, err
	}

	var workIssue *sdk.WorkIssue

	switch issueType {
	case "bug":
		workIssue, err = createIssue(logger, client, input, userManager, "")
		if err != nil {
			return nil, err
		}
	case "epic":
		workIssue, err = createMilestone(logger, httpClient, userManager, input, mutation.Project)
		if err != nil {
			return nil, err
		}
	default:
		workIssue, err = createIssue(logger, client, input, userManager, issueType)
		if err != nil {
			return nil, err
		}
	}

	return &sdk.MutationResponse{
		RefID:    sdk.StringPointer(workIssue.RefID),
		EntityID: sdk.StringPointer(workIssue.ID),
		URL:      sdk.StringPointer(workIssue.URL),
	}, nil
}

func createIssue(logger sdk.Logger, client sdk.GraphQLClient, input map[string]interface{}, userManager *UserManager, labelID string) (*sdk.WorkIssue, error) {
	var response struct {
		Data struct {
			CreateIssue struct {
				Issue CreateIssue `json:"issue"`
			} `json:"createIssue"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	sdk.LogDebug(logger, "sending issue creation mutation", "input", input)
	if labelID == "" {
		if err := client.Query(createIssueQuery, input, &response); err != nil {
			return nil, err
		}
	} else {
		input["label"] = labelID
		if err := client.Query(createIssueQueryWithLabels, input, &response); err != nil {
			return nil, err
		}
	}

	if len(response.Errors) > 0 {
		return nil, fmt.Errorf("error creating issue %v", response.Errors)
	}

	return response.Data.CreateIssue.Issue.toModel(logger, userManager, userManager.instanceid, userManager.customerID)
}

func getRefID(val sdk.MutationFieldValue) (string, error) {
	nameID, err := val.AsNameRefID()
	if err != nil {
		return "", fmt.Errorf("error decoding %s field as NameRefID: %w", val.Type.String(), err)
	}
	if nameID.RefID == nil {
		return "", errors.New("ref_id was omitted")
	}
	return *nameID.RefID, nil
}

func makeCreateMutation(logger sdk.Logger, projectRefID string, fields []sdk.MutationFieldValue) (map[string]interface{}, string, error) {

	if projectRefID == "" {
		return nil, "", errors.New("project ref id cannot be empty")
	}

	params := make(map[string]interface{})
	params["repositoryID"] = projectRefID

	var issueType string

	for _, fieldVal := range fields {
		switch fieldVal.RefID {
		case "issueType":
			iType, err := getRefID(fieldVal)
			if err != nil {
				return nil, "", fmt.Errorf("error decoding issue type field: %w", err)
			}
			issueType = iType
		case "title":
			title, err := fieldVal.AsString()
			if err != nil {
				return nil, "", fmt.Errorf("error decoding title field: %w", err)
			}
			params["title"] = title
		case "description":
			description, err := fieldVal.AsString()
			if err != nil {
				return nil, "", fmt.Errorf("error decoding description field: %w", err)
			}
			params["body"] = description
		case "epicDueDate":
			date, err := fieldVal.AsDate()
			if err != nil {
				return nil, "", fmt.Errorf("error decoding due date field: %w", err)
			}

			d := sdk.DateFromEpoch(date.Epoch)

			params["due_on"] = d.Format("2006-01-02T15:04:05Z")
		}
	}
	return params, issueType, nil
}

// CreateIssue create issue
type CreateIssue struct {
	RefID      string `json:"id"`
	Title      string `json:"title"`
	Body       string `json:"body"`
	Number     int    `json:"number"`
	URL        string `json:"url"`
	State      string `json:"state"`
	Repository struct {
		ID            string `json:"id"`
		NameWithOwner string `json:"nameWithOwner"`
	} `json:"repository"`
	CreatedAt time.Time    `json:"createdAt"`
	Author    *github.User `json:"author"`
}

func (c *CreateIssue) toModel(logger sdk.Logger, userManager *UserManager, integrationInstanceID string, customerID string) (*sdk.WorkIssue, error) {

	var issue issue

	issue.ID = c.RefID
	issue.CreatedAt = c.CreatedAt
	issue.UpdatedAt = c.CreatedAt
	issue.State = c.State
	issue.URL = c.URL
	issue.Title = c.Title
	issue.Body = c.Body
	issue.Number = c.Number
	issue.Author = userToAuthor(c.Author)
	projectID := sdk.NewWorkProjectID(customerID, c.Repository.ID, refType)
	return issue.ToModel(logger, userManager, customerID, integrationInstanceID, c.Repository.NameWithOwner, projectID)

}

const updateIssueQuery = `mutation updateIssue($id: ID!, %s) {
	updateIssue(input:{
	  id: $id,
	  %s
	}) {
	  issue {
			  id
			  title
			  number
			  url
			  state
			  repository {
				  id
				  nameWithOwner
			  }
			  createdAt
			  updatedAt
			  author {
				  login
			  }
			  milestone {
				  id
			  }
		  }
	}
  }
  `

func getUpdateQuery(includeTitle, includeMilestoneID, includeAssigneeIDs bool) string {

	filters := make([]string, 0)
	input := make([]string, 0)

	if includeTitle {
		filters = append(filters, "$title: String!")
		input = append(input, "title: $title")
	}

	if includeMilestoneID {
		filters = append(filters, "$epicID: ID!")
		input = append(input, "milestoneId: $epicID")
	}

	if includeAssigneeIDs {
		filters = append(filters, "$assignees: [ID!]")
		input = append(input, "assigneeIds: $assignees")
	}

	return fmt.Sprintf(updateIssueQuery, strings.Join(filters, ", "), strings.Join(input, ", "))
}

type issueUpdateResponse struct {
	Data struct {
		CreateIssue struct {
			Issue CreateIssue `json:"issue"`
		} `json:"updateIssue"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func (g *GithubIntegration) UpdateIssue(logger sdk.Logger, userManager *UserManager, issueRefID string, mutation *sdk.WorkIssueUpdateMutation, user sdk.MutationUser) (*sdk.MutationResponse, error) {

	var c sdk.Config
	c.APIKeyAuth = user.APIKeyAuth
	c.BasicAuth = user.BasicAuth
	c.OAuth2Auth = user.OAuth2Auth
	_, client, err := g.newGraphClient(logger, c)
	if err != nil {
		return nil, fmt.Errorf("error creating http client: %w", err)
	}

	var workIssue *sdk.WorkIssue
	var response *issueUpdateResponse

	input, hasMutation := makeIssueUpdate(mutation)
	if hasMutation {
		input["id"] = issueRefID

		query := getUpdateQuery(mutation.Set.Title != nil, mutation.Set.Epic != nil, mutation.Set.AssigneeRefID != nil)

		sdk.LogDebug(logger, "sending issue update mutation", "input", input, "user", user.RefID)
		if err := client.Query(query, input, &response); err != nil {
			return nil, err
		}

		if len(response.Errors) > 0 {
			return nil, fmt.Errorf("error creating issue %v", response.Errors)
		}

	}

	if mutation.Unset.Assignee || mutation.Unset.Epic {
		var err error
		response, err = unsetIssueFieldsIfAny(logger, client, issueRefID, mutation)
		if err != nil {
			return nil, err
		}
	}

	workIssue, err = response.Data.CreateIssue.Issue.toModel(logger, userManager, userManager.instanceid, userManager.customerID)
	if err != nil {
		return nil, err
	}

	return &sdk.MutationResponse{
		RefID:    sdk.StringPointer(issueRefID),
		EntityID: sdk.StringPointer(workIssue.ID),
		URL:      sdk.StringPointer(workIssue.URL),
	}, nil
}

func makeIssueUpdate(event *sdk.WorkIssueUpdateMutation) (input map[string]interface{}, hasMutation bool) {

	input = make(map[string]interface{})

	if event.Set.Title != nil {
		input["title"] = *event.Set.Title
		hasMutation = true
	}

	if event.Set.Epic != nil {
		input["epicID"] = *event.Set.Epic.RefID
		hasMutation = true
	}

	if event.Set.AssigneeRefID != nil {
		input["assignees"] = *event.Set.AssigneeRefID
		hasMutation = true
	}

	return input, hasMutation
}

const unsetIssueQuery = `mutation updateIssue($id: ID!){
	updateIssue(input:{
	  id:$id,
	  %s
	}) {
		issue {
			id
			title
			number
			url
			state
			repository {
				id
				nameWithOwner
			}
			createdAt
			updatedAt
			author {
				login
			}
			milestone {
				id
			}
		}
	}
  }
`

func unsetIssueFieldsIfAny(logger sdk.Logger, client sdk.GraphQLClient, issueRefID string, mutation *sdk.WorkIssueUpdateMutation) (*issueUpdateResponse, error) {

	var filters string

	if mutation.Unset.Epic && mutation.Unset.Assignee {
		filters = `assigneeIds: [], milestoneId: null`
	} else if mutation.Unset.Epic {
		filters = `milestoneId: null`
	} else if mutation.Unset.Assignee {
		filters = `assigneeIds: []`
	} else {
		return nil, nil // nothing to update
	}

	query := fmt.Sprintf(unsetIssueQuery, filters)

	input := make(map[string]interface{})
	input["id"] = issueRefID

	var r issueUpdateResponse

	err := client.Query(query, input, &r)
	if err != nil {
		return nil, err
	}

	if len(r.Errors) > 0 {
		return nil, fmt.Errorf("error updating issue %v", r.Errors)
	}

	return &r, nil
}
