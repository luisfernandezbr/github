package internal

import "github.com/pinpt/agent.next/pkg/util"

func toHTML(markdown string) string {
	return `<div class="source-github">` + util.ConvertMarkdownToHTML(markdown) + `</div>`
}
