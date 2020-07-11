package internal

import (
	"github.com/google/go-github/v32/github"
	"github.com/pinpt/agent.next/sdk"
)

func (g *GithubIntegration) fromPushEvent(logger sdk.Logger, client sdk.GraphQLClient, userManager *UserManager, control sdk.Control, customerID string, push *github.PushEvent) ([]*sdk.SourceCodeCommit, error) {
	commits := make([]*sdk.SourceCodeCommit, 0)
	for _, c := range push.Commits {
		var commit sdk.SourceCodeCommit
		commit.CustomerID = customerID
		commit.Active = true
		commit.RefType = refType
		commit.RefID = c.GetID()
		sha := c.GetSHA()
		if sha == "" {
			sha = c.GetID()
		}
		if push.GetDeleted() {
			commit.Excluded = true
		}
		commit.Sha = sha
		commit.Message = c.GetMessage()
		commit.URL = c.GetURL()
		commit.Identifier = push.GetRepo().GetFullName() + "#" + sha[0:7]
		commit.IntegrationInstanceID = sdk.StringPointer(userManager.instanceid)
		sdk.ConvertTimeToDateModel(c.GetTimestamp().Time, &commit.CreatedDate)

		if c.Author.GetLogin() == push.GetSender().GetLogin() {
			author := userToAuthor(push.GetSender())
			if err := userManager.emitAuthor(logger, author); err != nil {
				return nil, err
			}
			commit.AuthorRefID = author.RefID(customerID)
		} else {
			author := commitUserToAuthor(c.Author)
			if err := userManager.emitGitUser(logger, author); err != nil {
				return nil, err
			}
			commit.AuthorRefID = author.RefID(customerID)
		}

		if c.Committer.GetLogin() == push.GetSender().GetLogin() {
			committer := userToAuthor(push.GetSender())
			if err := userManager.emitAuthor(logger, committer); err != nil {
				return nil, err
			}
			commit.CommitterRefID = committer.RefID(customerID)
		} else {
			committer := commitUserToAuthor(c.Committer)
			if err := userManager.emitGitUser(logger, committer); err != nil {
				return nil, err
			}
			commit.CommitterRefID = committer.RefID(customerID)
		}

		repoID := sdk.NewSourceCodeRepoID(customerID, push.GetRepo().GetNodeID(), refType)
		commit.RepoID = repoID
		commit.ID = sdk.NewSourceCodeCommitID(customerID, sha, refType, repoID)

		commits = append(commits, &commit)
	}
	return commits, nil
}
