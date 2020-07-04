<div align="center">
	<img width="500" src=".github/logo.svg" alt="pinpt-logo">
</div>

<p align="center" color="#6a737d">
	<strong>This repo contains the official GitHub integration for Pinpoint</strong>
</p>


## Overview

This project contains the source code for the official GitHub integration for Pinpoint.

## Features

The following features are supported by this integration:

| Feature             | Export | WebHook | Notes                         |
|---------------------|:------:|:-------:|-------------------------------|
| Cloud               |   ✅   |    ✅   |                              |
| Self Service        |   ✅   |    ✅   |                              |
| Auth: Basic         |   ✅   |    ✅   |                              |
| Auth: API Key       |   ✅   |    ✅   |                              |
| Auth: OAuth2        |   ✅   |    ✅   |                              |
| Repo                |   ✅   |    ✅   |                              |
| Pull Request        |   ✅   |    ✅   |                              |
| Pull Comment        |   ✅   |    ✅   |                              |
| Pull Request Review |   ✅   |    ✅   |                              |
| Project             |   ✅   |    ✅   |                              |
| Epic                |   🛑   |    🛑   | No concept of Epics          |
| Sprint              |   🛑   |    🛑   | Repo projects act as Kanban  |
| Kanban              |   ✅   |    ✅   |                              |
| Issue               |   ✅   |    ✅   |                              |
| Issue Comment       |   ✅   |    ✅   |                              |
| Issue Type          |   ✅   |    ✅   | Built-in labels act as type  |
| Issue Status        |   ✅   |    ✅   | Open and Closed status only  |
| Issue Priority      |   🛑   |    🛑   | No concept of priority       |
| Issue Resolution    |   🛑   |    🛑   | No concept of resolution     |
| Issue Parent/Child  |   🛑   |    🛑   | No concept of parent/child   |
| Work Config         |   ✅   |    -    | Open and Closed states only  |
| Mutations           |   -    |    📝   | Partial / WIP                |
| Feed Notifications  |   🗓   |    🗓   | TODO                         |
| Builds              |   🗓   |    🗓   | TODO                         |
| Deployments         |   🗓   |    🗓   | TODO                         |
| Releases            |   🗓   |    🗓   | TODO                         |
| Security Events     |   🗓   |    🗓   | TODO                         |

## Running Locally

You can run locally to test against a repo with the following command (assuming you already have the Agent SDK installed):

```
agent dev . --log-level=debug --set "apikey_auth={\"apikey\":\"$GITHUB_TOKEN\"}" --set 'inclusions={"pinpt":"pinpt/agent"}'
```

Make sure you have the environment variable `GITHUB_TOKEN` set to a GitHub personal access token.  You can also change repositories by updating the `inclusions` array.  The key in the map should be the `organization` login value.

This will run an export for GitHub and print all the JSON objects to the console.

## License

This code is open source and licensed under the terms of the MIT License. Copyright &copy; 2020 by Pinpoint Software, Inc.
