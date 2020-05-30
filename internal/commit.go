package internal

type commitCommit struct {
	Sha string `json:"sha"`
}

type commit struct {
	Commit commitCommit `json:"commit"`
}

type commits struct {
	TotalCount int
	PageInfo   pageInfo
	Nodes      []commit
}
