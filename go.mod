module github.com/pinpt/agent.next.github

go 1.14

require (
	github.com/pinpt/agent.next v0.0.0-20200630032350-ba1895cc16fc
	github.com/pinpt/go-common/v10 v10.0.13
	golang.org/x/text v0.3.3 // indirect
	google.golang.org/grpc v1.30.0 // indirect
)

replace github.com/pinpt/agent.next => ../agent.next
