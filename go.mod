module github.com/pinpt/agent.next.github

go 1.14

require (
	github.com/google/go-github/v32 v32.0.0
	github.com/mailru/easyjson v0.7.1
	github.com/pinpt/agent.next v0.0.0-20200703195701-cfcbcce9e820
	github.com/pinpt/go-common/v10 v10.0.13
	github.com/stretchr/testify v1.6.1
	golang.org/x/text v0.3.3 // indirect
	google.golang.org/grpc v1.30.0 // indirect
)

replace github.com/pinpt/agent.next => ../agent.next
