module github.com/pinpt/agent.next.github

go 1.14

require (
	github.com/mailru/easyjson v0.7.1
	github.com/pinpt/agent.next v0.0.0-20200703152850-86e7650bd4da
	github.com/pinpt/go-common/v10 v10.0.13
	golang.org/x/text v0.3.3 // indirect
	google.golang.org/grpc v1.30.0 // indirect
)

replace github.com/pinpt/agent.next => ../agent.next
