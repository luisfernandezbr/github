module github.com/pinpt/agent.next.github

go 1.14

require (
	github.com/pinpt/agent.next v0.0.0-20200622210707-ffa31ea6895d
	github.com/pinpt/integration-sdk v0.0.1038 // indirect
	golang.org/x/crypto v0.0.0-20200622213623-75b288015ac9 // indirect
	golang.org/x/sys v0.0.0-20200622214017-ed371f2e16b4 // indirect
	golang.org/x/text v0.3.3 // indirect
	google.golang.org/grpc v1.30.0 // indirect
)

// TODO: this is only set while we're in rapid dev. once we get out of that we should remove
replace github.com/pinpt/agent.next => ../agent.next
