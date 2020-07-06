module github.com/pinpt/agent.next.github

go 1.14

require (
	github.com/go-redis/redis/v8 v8.0.0-beta.6 // indirect
	github.com/google/go-github/v32 v32.0.0
	github.com/mailru/easyjson v0.7.1
	github.com/mattn/go-colorable v0.1.7 // indirect
	github.com/pinpt/agent.next v0.0.0-20200705231546-bbe19f571b8b
	github.com/pinpt/go-common/v10 v10.0.14
	github.com/pinpt/httpclient v0.0.0-20200627153820-d374c2f15648 // indirect
	github.com/stretchr/testify v1.6.1
	golang.org/x/text v0.3.3 // indirect
)

replace github.com/pinpt/agent.next => ../agent.next
