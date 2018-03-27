// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markdown

import (
	"bytes"
	"strings"

	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/russross/blackfriday"
)

// Renderer is a extended version of underlying render object.
type Renderer struct {
	blackfriday.Renderer
	URLPrefix string
	IsWiki    bool
}

var byteMailto = []byte("mailto:")

// Link defines how formal links should be processed to produce corresponding HTML elements.
func (r *Renderer) Link(out *bytes.Buffer, link []byte, title []byte, content []byte) {
	// special case: this is not a link, a hash link or a mailto:, so it's a
	// relative URL
	if len(link) > 0 && !markup.IsLink(link) &&
		link[0] != '#' && !bytes.HasPrefix(link, byteMailto) {
		lnk := string(link)
		if r.IsWiki {
			lnk = util.URLJoin("wiki", lnk)
		}
		mLink := util.URLJoin(r.URLPrefix, lnk)
		link = []byte(mLink)
	}

	r.Renderer.Link(out, link, title, content)
}

// List renders markdown bullet or digit lists to HTML
func (r *Renderer) List(out *bytes.Buffer, text func() bool, flags int) {
	marker := out.Len()
	if out.Len() > 0 {
		out.WriteByte('\n')
	}

	if flags&blackfriday.LIST_TYPE_DEFINITION != 0 {
		out.WriteString("<dl>")
	} else if flags&blackfriday.LIST_TYPE_ORDERED != 0 {
		out.WriteString("<ol class='ui list'>")
	} else {
		out.WriteString("<ul class='ui list'>")
	}
	if !text() {
		out.Truncate(marker)
		return
	}
	if flags&blackfriday.LIST_TYPE_DEFINITION != 0 {
		out.WriteString("</dl>\n")
	} else if flags&blackfriday.LIST_TYPE_ORDERED != 0 {
		out.WriteString("</ol>\n")
	} else {
		out.WriteString("</ul>\n")
	}
}

// ListItem defines how list items should be processed to produce corresponding HTML elements.
func (r *Renderer) ListItem(out *bytes.Buffer, text []byte, flags int) {
	// Detect procedures to draw checkboxes.
	prefix := ""
	if bytes.HasPrefix(text, []byte("<p>")) {
		prefix = "<p>"
	}
	switch {
	case bytes.HasPrefix(text, []byte(prefix+"[ ] ")):
		text = append([]byte(`<span class="ui fitted disabled checkbox"><input type="checkbox" disabled="disabled" /><label /></span>`), text[3+len(prefix):]...)
		if prefix != "" {
			text = bytes.Replace(text, []byte(prefix), []byte{}, 1)
		}
	case bytes.HasPrefix(text, []byte(prefix+"[x] ")):
		text = append([]byte(`<span class="ui checked fitted disabled checkbox"><input type="checkbox" checked="" disabled="disabled" /><label /></span>`), text[3+len(prefix):]...)
		if prefix != "" {
			text = bytes.Replace(text, []byte(prefix), []byte{}, 1)
		}
	}
	r.Renderer.ListItem(out, text, flags)
}

// Note: this section is for purpose of increase performance and
// reduce memory allocation at runtime since they are constant literals.
var (
	svgSuffix         = []byte(".svg")
	svgSuffixWithMark = []byte(".svg?")
)

// Image defines how images should be processed to produce corresponding HTML elements.
func (r *Renderer) Image(out *bytes.Buffer, link []byte, title []byte, alt []byte) {
	prefix := r.URLPrefix
	if r.IsWiki {
		prefix = util.URLJoin(prefix, "wiki", "src")
	}
	prefix = strings.Replace(prefix, "/src/", "/raw/", 1)
	if len(link) > 0 {
		if markup.IsLink(link) {
			// External link with .svg suffix usually means CI status.
			// TODO: define a keyword to allow non-svg images render as external link.
			if bytes.HasSuffix(link, svgSuffix) || bytes.Contains(link, svgSuffixWithMark) {
				r.Renderer.Image(out, link, title, alt)
				return
			}
		} else {
			lnk := string(link)
			lnk = util.URLJoin(prefix, lnk)
			lnk = strings.Replace(lnk, " ", "+", -1)
			link = []byte(lnk)
		}
	}

	out.WriteString(`<a href="`)
	out.Write(link)
	out.WriteString(`">`)
	r.Renderer.Image(out, link, title, alt)
	out.WriteString("</a>")
}

const (
	blackfridayExtensions = 0 |
		blackfriday.EXTENSION_NO_INTRA_EMPHASIS |
		blackfriday.EXTENSION_TABLES |
		blackfriday.EXTENSION_FENCED_CODE |
		blackfriday.EXTENSION_STRIKETHROUGH |
		blackfriday.EXTENSION_NO_EMPTY_LINE_BEFORE_BLOCK
	blackfridayHTMLFlags = 0 |
		blackfriday.HTML_SKIP_STYLE |
		blackfriday.HTML_OMIT_CONTENTS |
		blackfriday.HTML_USE_SMARTYPANTS
)

// RenderRaw renders Markdown to HTML without handling special links.
func RenderRaw(body []byte, urlPrefix string, wikiMarkdown bool) []byte {
	renderer := &Renderer{
		Renderer:  blackfriday.HtmlRenderer(blackfridayHTMLFlags, "", ""),
		URLPrefix: urlPrefix,
		IsWiki:    wikiMarkdown,
	}

	exts := blackfridayExtensions
	if setting.Markdown.EnableHardLineBreak {
		exts |= blackfriday.EXTENSION_HARD_LINE_BREAK
	}

	body = blackfriday.Markdown(body, renderer, exts)
	return body
}

var (
	// MarkupName describes markup's name
	MarkupName = "markdown"
)

func init() {
	markup.RegisterParser(Parser{})
}

// Parser implements markup.Parser
type Parser struct {
}

// Name implements markup.Parser
func (Parser) Name() string {
	return MarkupName
}

// Extensions implements markup.Parser
func (Parser) Extensions() []string {
	return setting.Markdown.FileExtensions
}

// Render implements markup.Parser
func (Parser) Render(rawBytes []byte, urlPrefix string, metas map[string]string, isWiki bool) []byte {
	return RenderRaw(rawBytes, urlPrefix, isWiki)
}

// Render renders Markdown to HTML with all specific handling stuff.
func Render(rawBytes []byte, urlPrefix string, metas map[string]string) []byte {
	return markup.Render("a.md", rawBytes, urlPrefix, metas)
}

// RenderString renders Markdown to HTML with special links and returns string type.
func RenderString(raw, urlPrefix string, metas map[string]string) string {
	return markup.RenderString("a.md", raw, urlPrefix, metas)
}

// RenderWiki renders markdown wiki page to HTML and return HTML string
func RenderWiki(rawBytes []byte, urlPrefix string, metas map[string]string) string {
	return markup.RenderWiki("a.md", rawBytes, urlPrefix, metas)
}

// IsMarkdownFile reports whether name looks like a Markdown file
// based on its extension.
func IsMarkdownFile(name string) bool {
	return markup.IsMarkupFile(name, MarkupName)
}
