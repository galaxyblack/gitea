// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markdown_test

import (
	"strings"
	"testing"

	. "code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
)

const AppURL = "http://localhost:3000/"
const Repo = "gogits/gogs"
const AppSubURL = AppURL + Repo + "/"

func TestRender_StandardLinks(t *testing.T) {
	setting.AppURL = AppURL
	setting.AppSubURL = AppSubURL

	test := func(input, expected, expectedWiki string) {
		buffer := RenderString(input, setting.AppSubURL, nil)
		assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(string(buffer)))
		bufferWiki := RenderWiki([]byte(input), setting.AppSubURL, nil)
		assert.Equal(t, strings.TrimSpace(expectedWiki), strings.TrimSpace(bufferWiki))
	}

	googleRendered := `<p><a href="https://google.com/" rel="nofollow">https://google.com/</a></p>`
	test("<https://google.com/>", googleRendered, googleRendered)

	lnk := util.URLJoin(AppSubURL, "WikiPage")
	lnkWiki := util.URLJoin(AppSubURL, "wiki", "WikiPage")
	test("[WikiPage](WikiPage)",
		`<p><a href="`+lnk+`" rel="nofollow">WikiPage</a></p>`,
		`<p><a href="`+lnkWiki+`" rel="nofollow">WikiPage</a></p>`)
}

func TestMisc_IsMarkdownFile(t *testing.T) {
	setting.Markdown.FileExtensions = []string{".md", ".markdown", ".mdown", ".mkd"}
	trueTestCases := []string{
		"test.md",
		"wow.MARKDOWN",
		"LOL.mDoWn",
	}
	falseTestCases := []string{
		"test",
		"abcdefg",
		"abcdefghijklmnopqrstuvwxyz",
		"test.md.test",
	}

	for _, testCase := range trueTestCases {
		assert.True(t, IsMarkdownFile(testCase))
	}
	for _, testCase := range falseTestCases {
		assert.False(t, IsMarkdownFile(testCase))
	}
}

func TestRender_Images(t *testing.T) {
	setting.AppURL = AppURL
	setting.AppSubURL = AppSubURL

	test := func(input, expected string) {
		buffer := RenderString(input, setting.AppSubURL, nil)
		assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(string(buffer)))
	}

	url := "../../.images/src/02/train.jpg"
	title := "Train"
	result := util.URLJoin(AppSubURL, url)

	test(
		"!["+title+"]("+url+")",
		`<p><a href="`+result+`" rel="nofollow"><img src="`+result+`" alt="`+title+`"/></a></p>`)

	test(
		"[["+title+"|"+url+"]]",
		`<p><a href="`+result+`" rel="nofollow"><img src="`+result+`" title="`+title+`" alt="`+title+`"/></a></p>`)
}

func testAnswers(baseURLContent, baseURLImages string) []string {
	return []string{
		`<p>Wiki! Enjoy :)</p>

<ul>
<li><a href="` + baseURLContent + `/Links" rel="nofollow">Links, Language bindings, Engine bindings</a></li>
<li><a href="` + baseURLContent + `/Tips" rel="nofollow">Tips</a></li>
</ul>

<p>Ideas and codes</p>

<ul>
<li>Bezier widget (by <a href="` + AppURL + `r-lyeh" rel="nofollow">@r-lyeh</a>) <a href="http://localhost:3000/ocornut/imgui/issues/786" rel="nofollow">#786</a></li>
<li>Node graph editors <a href="https://github.com/ocornut/imgui/issues/306" rel="nofollow">https://github.com/ocornut/imgui/issues/306</a></li>
<li><a href="` + baseURLContent + `/memory_editor_example" rel="nofollow">Memory Editor</a></li>
<li><a href="` + baseURLContent + `/plot_var_example" rel="nofollow">Plot var helper</a></li>
</ul>
`,
		`<h2>What is Wine Staging?</h2>

<p><strong>Wine Staging</strong> on website <a href="http://wine-staging.com" rel="nofollow">wine-staging.com</a>.</p>

<h2>Quick Links</h2>

<p>Here are some links to the most important topics. You can find the full list of pages at the sidebar.</p>

<table>
<thead>
<tr>
<th><a href="` + baseURLImages + `/images/icon-install.png" rel="nofollow"><img src="` + baseURLImages + `/images/icon-install.png" title="icon-install.png" alt="images/icon-install.png"/></a></th>
<th><a href="` + baseURLContent + `/Installation" rel="nofollow">Installation</a></th>
</tr>
</thead>

<tbody>
<tr>
<td><a href="` + baseURLImages + `/images/icon-usage.png" rel="nofollow"><img src="` + baseURLImages + `/images/icon-usage.png" title="icon-usage.png" alt="images/icon-usage.png"/></a></td>
<td><a href="` + baseURLContent + `/Usage" rel="nofollow">Usage</a></td>
</tr>
</tbody>
</table>
`,
		`<p><a href="http://www.excelsiorjet.com/" rel="nofollow">Excelsior JET</a> allows you to create native executables for Windows, Linux and Mac OS X.</p>

<ol>
<li><a href="https://github.com/libgdx/libgdx/wiki/Gradle-on-the-Commandline#packaging-for-the-desktop" rel="nofollow">Package your libGDX application</a>
<a href="` + baseURLImages + `/images/1.png" rel="nofollow"><img src="` + baseURLImages + `/images/1.png" title="1.png" alt="images/1.png"/></a></li>
<li>Perform a test run by hitting the Run! button.
<a href="` + baseURLImages + `/images/2.png" rel="nofollow"><img src="` + baseURLImages + `/images/2.png" title="2.png" alt="images/2.png"/></a></li>
</ol>
`,
	}
}

// Test cases without ambiguous links
var sameCases = []string{
	// dear imgui wiki markdown extract: special wiki syntax
	`Wiki! Enjoy :)
- [[Links, Language bindings, Engine bindings|Links]]
- [[Tips]]

Ideas and codes

- Bezier widget (by @r-lyeh) ` + AppURL + `ocornut/imgui/issues/786
- Node graph editors https://github.com/ocornut/imgui/issues/306
- [[Memory Editor|memory_editor_example]]
- [[Plot var helper|plot_var_example]]`,
	// wine-staging wiki home extract: tables, special wiki syntax, images
	`## What is Wine Staging?
**Wine Staging** on website [wine-staging.com](http://wine-staging.com).

## Quick Links
Here are some links to the most important topics. You can find the full list of pages at the sidebar.

| [[images/icon-install.png]]    | [[Installation]]                                         |
|--------------------------------|----------------------------------------------------------|
| [[images/icon-usage.png]]      | [[Usage]]                                                |
`,
	// libgdx wiki page: inline images with special syntax
	`[Excelsior JET](http://www.excelsiorjet.com/) allows you to create native executables for Windows, Linux and Mac OS X.

1. [Package your libGDX application](https://github.com/libgdx/libgdx/wiki/Gradle-on-the-Commandline#packaging-for-the-desktop)
[[images/1.png]]
2. Perform a test run by hitting the Run! button.
[[images/2.png]]`,
}

func TestTotal_RenderWiki(t *testing.T) {
	answers := testAnswers(util.URLJoin(AppSubURL, "wiki/"), util.URLJoin(AppSubURL, "wiki", "raw/"))

	for i := 0; i < len(sameCases); i++ {
		line := RenderWiki([]byte(sameCases[i]), AppSubURL, nil)
		assert.Equal(t, answers[i], line)
	}

	testCases := []string{
		// Guard wiki sidebar: special syntax
		`[[Guardfile-DSL / Configuring-Guard|Guardfile-DSL---Configuring-Guard]]`,
		// rendered
		`<p><a href="` + AppSubURL + `wiki/Guardfile-DSL---Configuring-Guard" rel="nofollow">Guardfile-DSL / Configuring-Guard</a></p>
`,
		// special syntax
		`[[Name|Link]]`,
		// rendered
		`<p><a href="` + AppSubURL + `wiki/Link" rel="nofollow">Name</a></p>
`,
	}

	for i := 0; i < len(testCases); i += 2 {
		line := RenderWiki([]byte(testCases[i]), AppSubURL, nil)
		assert.Equal(t, testCases[i+1], line)
	}
}

func TestTotal_RenderString(t *testing.T) {
	answers := testAnswers(util.URLJoin(AppSubURL, "src", "master/"), util.URLJoin(AppSubURL, "raw", "master/"))

	for i := 0; i < len(sameCases); i++ {
		line := RenderString(sameCases[i], util.URLJoin(AppSubURL, "src", "master/"), nil)
		assert.Equal(t, answers[i], line)
	}

	testCases := []string{}

	for i := 0; i < len(testCases); i += 2 {
		line := RenderString(testCases[i], AppSubURL, nil)
		assert.Equal(t, testCases[i+1], line)
	}
}
