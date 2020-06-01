package internal

import (
	"fmt"
	"time"
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

type organization struct {
	Repositories repositories `json:"repositories"`
}

type allQueryResult struct {
	Organization organization `json:"organization"`
	RateLimit    rateLimit    `json:"rateLimit"`
}

var pullrequestPagedQuery = `
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

func generateAllDataQuery(before string, after string) string {
	var definitionLine, argLine string
	if before != "" {
		definitionLine = ", $before: String! "
		argLine = " before: $before "
	}
	if after != "" {
		definitionLine = ", $after: String! "
		argLine = " after: $after "
	}
	return fmt.Sprintf(`
	query GetAllData($login: String!, $first: Int! %s) {
		organization(login: $login) {
			repositories(first: $first %s isFork: false orderBy: {field: UPDATED_AT, direction: DESC}) {
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
