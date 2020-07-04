package internal

import (
	"fmt"
	"time"

	"github.com/pinpt/agent.next/sdk"
)

const refType = "github"

type pageInfo struct {
	HasNextPage bool   `json:"hasNextPage"`
	StartCursor string `json:"startCursor"`
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

type nameProp struct {
	Name string `json:"name"`
}

type oidProp struct {
	Oid string `json:"oid"`
}

var pullrequestFields = `
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
		type: __typename
		avatarUrl
		login
		url
		...on User {
			id
			email
			name
		}
	}
	author {
		type: __typename
		avatarUrl
		login
		url
		...on User {
			id
			email
			name
		}
	}
	commits(first: 10) {
		totalCount
		pageInfo {
			hasNextPage
			startCursor
			endCursor
		}
		edges {
			cursor
			node {
				commit {
					sha: oid
					message
					authoredDate
					additions
					deletions
					url
					author {
						avatarUrl
						email
						name
						user {
							id
							login
						}
					}
					committer {
						avatarUrl
						email
						name
						user {
							id
							login
						}
					}
				}
			}
		}
	}
	reviews(first: 10) {
		totalCount
		pageInfo {
			hasNextPage
			startCursor
			endCursor
		}
		edges {
			cursor
			node {
				id
				state
				createdAt
				url
				author {
					type: __typename
					avatarUrl
					login
					url
					...on User {
						id
						email
						name
					}
				}
			}
		}
	}
	comments(first: 10) {
		totalCount
		pageInfo {
			hasNextPage
			startCursor
			endCursor
		}
		edges {
			cursor
			node {
				id
				createdAt
				updatedAt
				url
				bodyHTML
				author {
					type: __typename
					avatarUrl
					login
					url
					...on User {
						id
						email
						name
					}
				}
			}
		}
	}
`

var pullrequestPagedQuery = fmt.Sprintf(`
query GetPullRequests($name: String!, $owner: String!, $first: Int!, $after: String) {
	repository(name: $name, owner: $owner) {
		pullRequests(first: $first, after: $after, orderBy: {field: UPDATED_AT, direction: DESC}) {
			totalCount
			pageInfo {
				hasNextPage
				startCursor
				endCursor
			}
			edges {
				cursor
				node {
					%s
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
`, pullrequestFields)

var pullrequestCommentsPagedQuery = `
query GetPullRequestComments($name: String!, $owner: String!, $first: Int!, $after: String, $number: Int!) {
	repository(name: $name, owner: $owner) {
		pullRequest(number: $number) {
			comments(first: $first, after: $after) {
				totalCount
				pageInfo {
					hasNextPage
					startCursor
					endCursor
				}
				edges {
					cursor
					node {
						id
						createdAt
						updatedAt
						url
						bodyHTML
						author {
							type: __typename
							avatarUrl
							login
							url
							...on User {
								id
								email
								name
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

var pullrequestReviewsPagedQuery = `
query GetPullRequestReviews($name: String!, $owner: String!, $first: Int!, $after: String, $number: Int!) {
	repository(name: $name, owner: $owner) {
		pullRequest(number: $number) {
			reviews(first: $first, after: $after) {
				totalCount
				pageInfo {
					hasNextPage
					startCursor
					endCursor
				}
				edges {
					cursor
					node {
						id
						state
						createdAt
						url
						author {
							type: __typename
							avatarUrl
							login
							url
							...on User {
								id
								email
								name
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

type allOrgViewOrg struct {
	Organizations organizations `json:"organizations"`
}

type allOrgsResult struct {
	Viewer    allOrgViewOrg `json:"viewer"`
	RateLimit rateLimit     `json:"rateLimit"`
}

type org struct {
	Name     string `json:"name"`
	Login    string `json:"login"`
	IsMember bool   `json:"viewerIsAMember"`
	IsAdmin  bool   `json:"viewerCanAdminister"`
}

type organizations struct {
	Nodes []org `json:"nodes"`
}

func generateAllPRCommitsQuery(before string, after string) string {
	var definitionLine, argLine string
	if before != "" {
		definitionLine = ", $before: String! "
		argLine = " before: $before "
	}
	if after != "" {
		definitionLine = ", $after: String! "
		argLine = " after: $after "
	}
	return fmt.Sprintf(`query GetAllPRCommits($id: ID!, $first: Int! %s) {
	node(id: $id) {
		...on PullRequest {
			commits(first: $first %s) {
				totalCount
				pageInfo {
					hasNextPage
					startCursor
					endCursor
				}
				edges {
					cursor
					node {
						commit {
							sha: oid
							message
							authoredDate
							additions
							deletions
							url
							author {
								avatarUrl
								email
								name
								user {
									id
									login
								}
							}
							committer {
								avatarUrl
								email
								name
								user {
									id
									login
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
`, definitionLine, argLine)
}

var allOrgsQuery = `
query GetAllOrgs($first: Int!) {
	viewer {
		organizations(first: $first) {
			nodes {
				name
				login
				viewerIsAMember
				viewerCanAdminister
			}
		}
	}
}
`

type viewerResult struct {
	Viewer struct {
		Login string `json:"login"`
	} `json:"viewer"`
}

func generateViewerLogin() string {
	return `query viewer {
		viewer {
		  login
		}
	 }`
}

type repoName struct {
	ID         string                `json:"id"`
	RepoName   string                `json:"name"`
	Name       string                `json:"nameWithOwner"`
	IsPrivate  bool                  `json:"isPrivate"`
	IsArchived bool                  `json:"isArchived"`
	Scope      sdk.ConfigAccountType `json:"-"`
	Login      string                `json:"-"`
}

type repoWithNameResult struct {
	Data struct {
		Repositories struct {
			TotalCount int        `json:"totalCount"`
			PageInfo   pageInfo   `json:"pageInfo"`
			Nodes      []repoName `json:"nodes"`
		} `json:"repositories"`
	} `json:"data"`
	RateLimit rateLimit `json:"rateLimit"`
}

func generateAllReposQuery(after string, scope string) string {
	var definitionLine, argLine string
	if after != "" {
		definitionLine = ", $after: String! "
		argLine = " after: $after "
	}
	return fmt.Sprintf(`
	query GetAllRepos($login: String!, $first: Int! %[1]s) {
		data: %[3]s(login: $login) {
			repositories(first: $first %[2]s orderBy: {field: PUSHED_AT, direction: DESC}) {
				totalCount
				pageInfo {
					hasNextPage
					startCursor
					endCursor
				}
				nodes {
					id
					name
					nameWithOwner
					isPrivate
					isArchived
				}
			}
		}
		rateLimit {
			limit
			cost
			remaining
			resetAt
		}
	}`, definitionLine, argLine, scope)
}

func getAllRepoDataQuery(owner, name, label, cursor string) string {
	var cursorVal string
	if cursor != "" {
		cursorVal = fmt.Sprintf(`, after: "%s"`, cursor)
	}
	return fmt.Sprintf(`
%s: repository(name: "%s", owner: "%s") {
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
		isFork
		owner {
			login
		}
		pullRequests(first: 10, orderBy: {field: UPDATED_AT, direction: DESC} %s) {
			totalCount
			pageInfo {
				hasNextPage
				startCursor
				endCursor
			}
			edges {
				cursor
				node {
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
						type: __typename
						avatarUrl
						login
						url
						... on User {
							id
							email
							name
						}
					}
					author {
						type: __typename
						avatarUrl
						login
						url
						... on User {
							id
							email
							name
						}
					}
					timelineItems(last: 1, itemTypes: CLOSED_EVENT) {
						nodes {
							... on ClosedEvent {
								actor {
									type: __typename
									avatarUrl
									login
									url
									... on User {
										id
										email
										name
									}
								}
							}
						}
					}
					commits(first: 10) {
						totalCount
						pageInfo {
							hasNextPage
							startCursor
							endCursor
						}
						edges {
							cursor
							node {
								commit {
									sha: oid
									message
									authoredDate
									additions
									deletions
									url
									author {
										avatarUrl
										email
										name
										user {
											id
											login
										}
									}
									committer {
										avatarUrl
										email
										name
										user {
											id
											login
										}
									}
								}
							}
						}
					}
					reviews(first: 10) {
						totalCount
						pageInfo {
							hasNextPage
							startCursor
							endCursor
						}
						edges {
						cursor
							node {
								id
								state
								createdAt
								url
								author {
									type: __typename
									avatarUrl
									login
									url
									... on User {
										id
										email
										name
									}
								}
							}
						}
					}
					comments(first: 10) {
						totalCount
						pageInfo {
							hasNextPage
							startCursor
							endCursor
						}
						edges {
							cursor
							node {
								id
								createdAt
								updatedAt
								url
								bodyHTML
								author {
									type: __typename
									avatarUrl
									login
									url
									... on User {
										id
										email
										name
									}
								}
							}
						}
					}
				}
			}
		}
	}`, label, name, owner, cursorVal)
}

var pullrequestNodeIDQuery = `
query getPRNodeID($name: String!, $owner: String!, $number: Int!) { 
	repository(name: $name owner: $owner){
	  pullRequest(number:$number) {
		 id
	  }
	}
}`

type mutationResponse struct {
	ID int `json:"clientMutationId"`
}

var pullRequestUpdateMutation = `
mutation updatePullrequest($input: UpdatePullRequestInput!) {
	updatePullRequest(input: $input) {
		clientMutationId
	}
}
`
