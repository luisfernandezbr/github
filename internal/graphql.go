package internal

import (
	"fmt"
	"time"

	"github.com/pinpt/go-common/datamodel"
	"github.com/pinpt/go-common/datetime"
	"github.com/pinpt/integration-sdk/sourcecode"
)

const refType = "github"

type pageInfo struct {
	HasNextPage bool   `json:"hasNextPage"`
	EndCursor   string `json:"endCursor"`
}

type rateLimit struct {
	Limit     int       `json:"limit"`
	Cost      int       `json:"cost"`
	Remaining int       `json:"remaining"`
	ResetAt   time.Time `json:"resetAt"`
}

func (l rateLimit) ShouldPause() bool {
	// stop at 80%
	return float32(l.Remaining)*.8 >= float32(l.Limit)
}

type author struct {
	ID     string  `json:"id"`
	Email  string  `json:"email"`
	Name   string  `json:"name"`
	Avatar string  `json:"avatarUrl"`
	Login  string  `json:"login"`
	BotURL *string `json:"boturl"`
}

func (a author) RefID() string {
	// FIXME: this doesn't seem right but tried to following current agent
	// https://github.com/pinpt/agent/blob/afcc3e5b585a1902eeeaec89e37424f651818e6f/integrations/github/user.go#L199
	if a.Name == "GitHub" && a.Email == "noreply@github.com" {
		return "github-noreply"
	}
	if a.Login == "" {
		return ""
	}
	return a.Login
}

type nameProp struct {
	Name string `json:"name"`
}

type oidProp struct {
	Oid string `json:"oid"`
}

type repository struct {
	ID            string       `json:"id"`
	Name          string       `json:"nameWithOwner"`
	URL           string       `json:"url"`
	UpdatedAt     time.Time    `json:"updatedAt"`
	Description   string       `json:"description"`
	Language      nameProp     `json:"primaryLanguage"`
	DefaultBranch nameProp     `json:"defaultBranchRef"`
	IsArchived    bool         `json:"isArchived"`
	Pullrequests  pullrequests `json:"pullRequests"`
}

func (r repository) ToModel(customerID string) datamodel.Model {
	repo := &sourcecode.Repo{}
	repo.ID = sourcecode.NewRepoID(customerID, refType, repo.ID)
	repo.CustomerID = customerID
	repo.Name = r.Name
	repo.Description = r.Description
	repo.RefID = r.ID
	repo.RefType = refType
	repo.Language = r.Language.Name
	repo.DefaultBranch = r.DefaultBranch.Name
	repo.URL = r.URL
	repo.UpdatedAt = datetime.TimeToEpoch(r.UpdatedAt)
	repo.Active = !r.IsArchived
	return repo
}

type commitCommit struct {
	Sha string `json:"sha"`
}

type commit struct {
	Commit commitCommit `json:"commit"`
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

type pullrequest struct {
	ID          string    `json:"id"`
	Body        string    `json:"bodyHTML"`
	URL         string    `json:"url"`
	Closed      bool      `json:"closed"`
	Draft       bool      `json:"draft"`
	Locked      bool      `json:"locked"`
	Merged      bool      `json:"merged"`
	Number      int       `json:"number"`
	State       string    `json:"state"`
	Title       string    `json:"title"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	MergedAt    time.Time `json:"mergedAt"`
	Author      author    `json:"author"`
	Branch      string    `json:"branch"`
	MergeCommit oidProp   `json:"mergeCommit"`
	MergedBy    author    `json:"mergedBy"`
	Commits     commits   `json:"commits"`
	Reviews     reviews   `json:"reviews"`
}

func (pr pullrequest) ToModel(customerID string, repoName string, repoID string) datamodel.Model {
	// FIXME: implement the remaining fields
	pullrequest := &sourcecode.PullRequest{}
	pullrequest.ID = sourcecode.NewPullRequestID(customerID, pr.ID, refType, repoID)
	pullrequest.CustomerID = customerID
	pullrequest.RepoID = repoID
	pullrequest.RefID = pr.ID
	pullrequest.RefType = refType
	pullrequest.Title = pr.Title
	pullrequest.URL = pr.URL
	pullrequest.Description = pr.Body
	pullrequest.Draft = pr.Draft
	commitids := []string{}
	commitshas := []string{}
	pullrequest.CreatedByRefID = pr.Author.RefID()
	for _, node := range pr.Commits.Nodes {
		commitshas = append(commitshas, node.Commit.Sha)
		commitids = append(commitids, sourcecode.NewCommitID(customerID, node.Commit.Sha, refType, repoID))
	}
	pullrequest.CommitShas = commitshas
	pullrequest.CommitIds = commitids
	if len(commitids) > 0 {
		pullrequest.BranchID = sourcecode.NewBranchID(refType, repoID, customerID, pr.Branch, commitids[0])
	} else {
		pullrequest.BranchID = sourcecode.NewBranchID(refType, repoID, customerID, pr.Branch, "")
	}
	pullrequest.BranchName = pr.Branch
	pullrequest.Identifier = fmt.Sprintf("%s#%d", repoName, pr.Number)
	if pr.Merged {
		pullrequest.MergeSha = pr.MergeCommit.Oid
		md, _ := datetime.NewDateWithTime(pr.MergedAt)
		pullrequest.MergedDate = sourcecode.PullRequestMergedDate{
			Epoch:   md.Epoch,
			Rfc3339: md.Rfc3339,
			Offset:  md.Offset,
		}
		pullrequest.MergedByRefID = pr.MergedBy.RefID()
	}
	if pr.Locked {
		pullrequest.Status = sourcecode.PullRequestStatusLocked
	} else {
		switch pr.State {
		case "OPEN":
			pullrequest.Status = sourcecode.PullRequestStatusOpen
		case "CLOSED":
			pullrequest.Status = sourcecode.PullRequestStatusClosed
			pullrequest.ClosedByRefID = "" // TODO
		case "MERGED":
			pullrequest.Status = sourcecode.PullRequestStatusMerged
		}
	}
	cd, _ := datetime.NewDateWithTime(pr.CreatedAt)
	pullrequest.CreatedDate = sourcecode.PullRequestCreatedDate{
		Epoch:   cd.Epoch,
		Rfc3339: cd.Rfc3339,
		Offset:  cd.Offset,
	}
	ud, _ := datetime.NewDateWithTime(pr.UpdatedAt)
	pullrequest.UpdatedDate = sourcecode.PullRequestUpdatedDate{
		Epoch:   ud.Epoch,
		Rfc3339: ud.Rfc3339,
		Offset:  ud.Offset,
	}
	return pullrequest
}

type repositoryPullrequests struct {
	Repository repository `json:"repository"`
	RateLimit  rateLimit  `json:"rateLimit"`
}

type pullrequests struct {
	TotalCount int
	PageInfo   pageInfo
	Nodes      []pullrequest
}

type repositories struct {
	TotalCount int
	PageInfo   pageInfo
	Nodes      []repository
}

type commits struct {
	TotalCount int
	PageInfo   pageInfo
	Nodes      []commit
}

type reviews struct {
	TotalCount int
	PageInfo   pageInfo
	Nodes      []review
}

type organization struct {
	Repositories repositories `json:"repositories"`
}

type allQueryResult struct {
	Organization organization `json:"organization"`
	RateLimit    rateLimit    `json:"rateLimit"`
}

var pullrequestPageQuery = `
query GetPullRequests($name: String!, $owner: String!, $first: Int!, $after: String) {
	repository(name: $name, owner: $owner) {
		pullRequests(first: $first, after: $after, orderBy: {field: UPDATED_AT, direction: DESC}) {
			totalCount
			pageInfo {
				hasNextPage
				endCursor
			}
			nodes {
				id
				bodyHTML
				url
				closed
				draft: isDraft
				locked
				merged
				number
				state
				title
				createdAt
				updatedAt
				mergedAt
				branch: headRefName
				mergeCommit {
					oid
				}
				mergedBy {
					avatarUrl
					login
					...on User {
						id
						email
						name
					}
					...on Bot {
						id
						boturl: url
					}
				}
				author {
					avatarUrl
					login
					...on User {
						id
						email
						name
					}
					...on Bot {
						id
						boturl: url
					}
				}
				commits(first:100) {
					totalCount
					pageInfo {
						hasNextPage
						endCursor
					}
					nodes {
						commit {
							sha: oid
						}
					}
				}
				reviews(first: 10) {
					nodes {
						id
						state
						createdAt
						url
						author {
							avatarUrl
							login
							...on User {
								id
								email
								name
							}
							...on Bot {
								id
								boturl: url
							}
						}
					}
				}
			}
		}
	}
	rateLimit {
		limit
		cost
		remaining
		resetAt
	}
}
`

var allDataQuery = `
query GetAllData($login: String!, $first: Int!, $after: String) {
	organization(login: $login) {
		repositories(first: $first, after: $after, isFork: false, orderBy: {field: UPDATED_AT, direction: DESC}) {
			totalCount
			pageInfo {
				hasNextPage
				endCursor
			}
			nodes {
				id
				nameWithOwner
				url
				updatedAt
				description
				defaultBranchRef {
					name
				}
				primaryLanguage {
					name
				}
				isArchived

				pullRequests(first: 10, orderBy: {field: UPDATED_AT, direction: DESC}) {
					totalCount
					pageInfo {
						hasNextPage
						endCursor
					}
					nodes {
						id
						bodyHTML
						url
						closed
						draft: isDraft
						locked
						merged
						number
						state
						title
						createdAt
						updatedAt
						mergedAt
						branch: headRefName
						mergeCommit {
							oid
						}
						mergedBy {
							avatarUrl
							login
							...on User {
								id
								email
								name
							}
							...on Bot {
								id
								boturl: url
							}
						}		
						author {
							avatarUrl
							login
							...on User {
								id
								email
								name
							}
							...on Bot {
								id
								boturl: url
							}
						}
						commits(first:100) {
							totalCount
							pageInfo {
								hasNextPage
								endCursor
							}
							nodes {
								commit {
									sha: oid
								}
							}
						}
						reviews(first: 10) {
							nodes {
								id
								state
								createdAt
								url
								author {
									avatarUrl
									login
									...on User {
										id
										email
										name
									}
									...on Bot {
										id
										boturl: url
									}
								}
							}
						}
					}
				}
			}
		}
	}
	rateLimit {
		limit
		cost
		remaining
		resetAt
	}
}
`

var _allDataQuery = `
query GetAllData($login: String!, $first: Int!) {
	organization(login: $login) {
	  repositories(first: $first, isFork: false, orderBy: {field: UPDATED_AT, direction: DESC}) {
		totalCount
		pageInfo {
		  hasNextPage
		  endCursor
		}
		edges {
		  node {
			pullRequests(first: 10, orderBy: {field: UPDATED_AT, direction: DESC}) {
			  totalCount
			  pageInfo {
				hasNextPage
				endCursor
			  }
			  edges {
				node {
				  id
				  url
				  reviews(first: 10) {
					totalCount
					pageInfo {
					  hasNextPage
					  endCursor
					}
					edges {
					  node {
						id
						state
						createdAt
						author {
						  login
						  avatarUrl
						  url
						}
					  }
					}
				  }
				  comments(first: 10) {
					totalCount
					pageInfo {
					  hasNextPage
					  endCursor
					}
					edges {
					  node {
						id
						createdAt
						bodyHTML
						author {
						  login
						  avatarUrl
						  url
						}
					  }
					}
				  }
				  commits(first: 10) {
					totalCount
					pageInfo {
					  hasNextPage
					  endCursor
					}
					edges {
					  node {
						id
						url
						commit {
						  sha: oid
						  message
						  additions
						  deletions
						  authoredDate
						  url
						  author {
							user {
							  id
							  login
							  avatarUrl
							  name
							}
							email
						  }
						}
					  }
					}
				  }
				}
			  }
			}
		  }
		}
	  }
	}
  }
`
