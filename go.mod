module github.com/pinpt/agent.next.github

go 1.14

require (
	github.com/google/go-github/v32 v32.0.0
	github.com/mailru/easyjson v0.7.1
	github.com/pinpt/agent.next v0.0.0-20200911210317-929ce86d78f8
	github.com/stretchr/testify v1.6.1
	golang.org/x/net v0.0.0-20200707034311-ab3426394381 // indirect
)

replace github.com/pinpt/agent.next => ../agent.next
