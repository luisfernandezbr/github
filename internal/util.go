package internal

import "github.com/pinpt/agent/sdk"

func toHTML(markdown string) string {
	return `<div class="source-github">` + sdk.ConvertMarkdownToHTML(markdown) + `</div>`
}
