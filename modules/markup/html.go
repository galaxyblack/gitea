// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markup

import (
	"bytes"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/Unknwon/com"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Issue name styles
const (
	IssueNameStyleNumeric      = "numeric"
	IssueNameStyleAlphanumeric = "alphanumeric"
)

var (
	// NOTE: All below regex matching do not perform any extra validation.
	// Thus a link is produced even if the linked entity does not exist.
	// While fast, this is also incorrect and lead to false positives.
	// TODO: fix invalid linking issue

	// mentionPattern matches all mentions in the form of "@user"
	mentionPattern = regexp.MustCompile(`(?:\s|^|\W)(@[0-9a-zA-Z-_\.]+)`)

	// issueNumericPattern matches string that references to a numeric issue, e.g. #1287
	issueNumericPattern = regexp.MustCompile(`(?:\s|^|\W)(#[0-9]+)\b`)
	// issueAlphanumericPattern matches string that references to an alphanumeric issue, e.g. ABC-1234
	issueAlphanumericPattern = regexp.MustCompile(`(?:\s|^|\W)([A-Z]{1,10}-[1-9][0-9]*)\b`)
	// crossReferenceIssueNumericPattern matches string that references a numeric issue in a different repository
	// e.g. gogits/gogs#12345
	crossReferenceIssueNumericPattern = regexp.MustCompile(`(?:\s|^|\W)([0-9a-zA-Z-_\.]+/[0-9a-zA-Z-_\.]+#[0-9]+)\b`)

	// sha1CurrentPattern matches string that represents a commit SHA, e.g. d8a994ef243349f321568f9e36d5c3f444b99cae
	// Although SHA1 hashes are 40 chars long, the regex matches the hash from 7 to 40 chars in length
	// so that abbreviated hash links can be used as well. This matches git and github useability.
	sha1CurrentPattern = regexp.MustCompile(`(?:\s|^|\W)([0-9a-f]{7,40})\b`)

	// shortLinkPattern matches short but difficult to parse [[name|link|arg=test]] syntax
	shortLinkPattern = regexp.MustCompile(`\[\[(.*?)\]\](\w*)`)

	// anySHA1Pattern allows to split url containing SHA into parts
	anySHA1Pattern = regexp.MustCompile(`https?://(?:\S+/){4}([0-9a-f]{40})/?([^#\s]+)?(?:#(\S+))?`)

	validLinksPattern = regexp.MustCompile(`^[a-z][\w-]+://`)

	// While this email regex is definitely not perfect and I'm sure you can come up
	// with edge cases, it is still accepted by the CommonMark specification, as
	// well as the HTML5 spec:
	//   http://spec.commonmark.org/0.28/#email-address
	//   https://html.spec.whatwg.org/multipage/input.html#e-mail-state-(type%3Demail)
	emailRegex = regexp.MustCompile("[a-zA-Z0-9.!#$%&'*+\\/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*")

	// matches http/https links. used for autlinking those. partly modified from
	// the original present in autolink.js
	linkRegex = regexp.MustCompile(`(?:(?:http|https):\/\/(?:[\-;:&=\+\$,\w]+@)?[A-Za-z0-9\.\-]+|(?:www\.|[\-;:&=\+\$,\w]+@)[A-Za-z0-9\.\-]+)(?:(?:\/[\+~%\/\.\w\-]*)?\??(?:[\-\+:=&;%@\.\w]*)#?(?:[\.\!\/\\\w]*))?`)
)

// regexp for full links to issues/pulls
var issueFullPattern *regexp.Regexp

// IsLink reports whether link fits valid format.
func IsLink(link []byte) bool {
	return isLink(link)
}

// isLink reports whether link fits valid format.
func isLink(link []byte) bool {
	return validLinksPattern.Match(link)
}

func isLinkStr(link string) bool {
	return validLinksPattern.MatchString(link)
}

func getIssueFullPattern() *regexp.Regexp {
	if issueFullPattern == nil {
		appURL := setting.AppURL
		if len(appURL) > 0 && appURL[len(appURL)-1] != '/' {
			appURL += "/"
		}
		issueFullPattern = regexp.MustCompile(appURL +
			`\w+/\w+/(?:issues|pulls)/((?:\w{1,10}-)?[1-9][0-9]*)([\?|#]\S+.(\S+)?)?\b`)
	}
	return issueFullPattern
}

// FindAllMentions matches mention patterns in given content
// and returns a list of found user names without @ prefix.
func FindAllMentions(content string) []string {
	mentions := mentionPattern.FindAllStringSubmatch(content, -1)
	ret := make([]string, len(mentions))
	for i, val := range mentions {
		ret[i] = val[1][1:]
	}
	return ret
}

// cutoutVerbosePrefix cutouts URL prefix including sub-path to
// return a clean unified string of request URL path.
func cutoutVerbosePrefix(prefix string) string {
	if len(prefix) == 0 || prefix[0] != '/' {
		return prefix
	}
	count := 0
	for i := 0; i < len(prefix); i++ {
		if prefix[i] == '/' {
			count++
		}
		if count >= 3+setting.AppSubURLDepth {
			return prefix[:i]
		}
	}
	return prefix
}

// IsSameDomain checks if given url string has the same hostname as current Gitea instance
func IsSameDomain(s string) bool {
	if strings.HasPrefix(s, "/") {
		return true
	}
	if uapp, err := url.Parse(setting.AppURL); err == nil {
		if u, err := url.Parse(s); err == nil {
			return u.Host == uapp.Host
		}
		return false
	}
	return false
}

type postProcessError struct {
	context string
	err     error
}

func (p *postProcessError) Error() string {
	return "PostProcess: " + p.context + ", " + p.Error()
}

type processor func(ctx *postProcessCtx, node *html.Node)

var defaultProcessors = []processor{
	mentionProcessor,
	shortLinkProcessor,
	fullIssuePatternProcessor,
	issueIndexPatternProcessor,
	crossReferenceIssueIndexPatternProcessor,
	fullSha1PatternProcessor,
	sha1CurrentPatternProcessor,
	emailAddressProcessor,
	linkProcessor,
}

type postProcessCtx struct {
	metas          map[string]string
	urlPrefix      string
	isWikiMarkdown bool

	// processors used by this context.
	procs []processor

	// if set to true, when an <a> is found, instead of just returning during
	// visitNode, it will recursively visit the node exclusively running
	// shortLinkProcessorFull with true.
	visitLinksForShortLinks bool
}

// PostProcess does the final required transformations to the passed raw HTML
// data, and ensures its validity. Transformations include: replacing links and
// emails with HTML links, parsing shortlinks in the format of [[Link]], like
// MediaWiki, linking issues in the format #ID, and mentions in the format
// @user, and others.
func PostProcess(
	rawHTML []byte,
	urlPrefix string,
	metas map[string]string,
	isWikiMarkdown bool,
) ([]byte, error) {
	// create the context from the parameters
	ctx := &postProcessCtx{
		metas:          metas,
		urlPrefix:      urlPrefix,
		isWikiMarkdown: isWikiMarkdown,
		procs:          defaultProcessors,
		visitLinksForShortLinks: true,
	}
	return ctx.postProcess(rawHTML)
}

var commitMessageProcessors = []processor{
	mentionProcessor,
	fullIssuePatternProcessor,
	issueIndexPatternProcessor,
	crossReferenceIssueIndexPatternProcessor,
	fullSha1PatternProcessor,
	sha1CurrentPatternProcessor,
	emailAddressProcessor,
	linkProcessor,
}

// RenderCommitMessage will use the same logic as PostProcess, but will disable
// the shortLinkProcessor and will add a defaultLinkProcessor if defaultLink is
// set, which changes every text node into a link to the passed default link.
func RenderCommitMessage(
	rawHTML []byte,
	urlPrefix, defaultLink string,
	metas map[string]string,
) ([]byte, error) {
	ctx := &postProcessCtx{
		metas:     metas,
		urlPrefix: urlPrefix,
		procs:     commitMessageProcessors,
	}
	if defaultLink != "" {
		// we don't have to fear data races, because being
		// commitMessageProcessors of fixed len and cap, every time we append
		// something to it the slice is realloc+copied, so append always
		// generates the slice ex-novo.
		ctx.procs = append(ctx.procs, genDefaultLinkProcessor(defaultLink))
	}
	return ctx.postProcess(rawHTML)
}

var byteBodyTag = []byte("<body>")
var byteBodyTagClosing = []byte("</body>")

func (ctx *postProcessCtx) postProcess(rawHTML []byte) ([]byte, error) {
	if ctx.procs == nil {
		ctx.procs = defaultProcessors
	}

	// give a generous extra 50 bytes
	res := make([]byte, 0, len(rawHTML)+50)
	res = append(res, byteBodyTag...)
	res = append(res, rawHTML...)
	res = append(res, byteBodyTagClosing...)

	// parse the HTML
	nodes, err := html.ParseFragment(bytes.NewReader(res), nil)
	if err != nil {
		return nil, &postProcessError{"invalid HTML", err}
	}

	for _, node := range nodes {
		ctx.visitNode(node)
	}

	// Create buffer in which the data will be placed again. We know that the
	// length will be at least that of res; to spare a few alloc+copy, we
	// reuse res, resetting its length to 0.
	buf := bytes.NewBuffer(res[:0])
	// Render everything to buf.
	for _, node := range nodes {
		err = html.Render(buf, node)
		if err != nil {
			return nil, &postProcessError{"error rendering processed HTML", err}
		}
	}

	// remove initial parts - because Render creates a whole HTML page.
	res = buf.Bytes()
	res = res[bytes.Index(res, byteBodyTag)+len(byteBodyTag) : bytes.LastIndex(res, byteBodyTagClosing)]

	// Everything done successfully, return parsed data.
	return res, nil
}

func (ctx *postProcessCtx) visitNode(node *html.Node) {
	// We ignore code, pre and already generated links.
	switch node.Type {
	case html.TextNode:
		ctx.textNode(node)
	case html.ElementNode:
		if node.Data == "a" || node.Data == "code" || node.Data == "pre" {
			if node.Data == "a" && ctx.visitLinksForShortLinks {
				ctx.visitNodeForShortLinks(node)
			}
			return
		}
		for n := node.FirstChild; n != nil; n = n.NextSibling {
			ctx.visitNode(n)
		}
	}
	// ignore everything else
}

func (ctx *postProcessCtx) visitNodeForShortLinks(node *html.Node) {
	switch node.Type {
	case html.TextNode:
		shortLinkProcessorFull(ctx, node, true)
	case html.ElementNode:
		if node.Data == "code" || node.Data == "pre" {
			return
		}
		for n := node.FirstChild; n != nil; n = n.NextSibling {
			ctx.visitNodeForShortLinks(n)
		}
	}
}

// textNode runs the passed node through various processors, in order to handle
// all kinds of special links handled by the post-processing.
func (ctx *postProcessCtx) textNode(node *html.Node) {
	for _, processor := range ctx.procs {
		processor(ctx, node)
	}
}

func createLink(href, content string) *html.Node {
	textNode := &html.Node{
		Type: html.TextNode,
		Data: content,
	}
	linkNode := &html.Node{
		FirstChild: textNode,
		LastChild:  textNode,
		Type:       html.ElementNode,
		Data:       "a",
		DataAtom:   atom.A,
		Attr: []html.Attribute{
			{Key: "href", Val: href},
		},
	}
	textNode.Parent = linkNode
	return linkNode
}

// replaceContent takes a text node, and in its content it replaces a section of
// it with the specified newNode. An example to visualize how this can work can
// be found here: https://play.golang.org/p/5zP8NnHZ03s
func replaceContent(node *html.Node, i, j int, newNode *html.Node) {
	// get the data before and after the match
	before := node.Data[:i]
	after := node.Data[j:]

	// Replace in the current node the text, so that it is only what it is
	// supposed to have.
	node.Data = before

	// Get the current next sibling, before which we place the replaced data,
	// and after that we place the new text node.
	nextSibling := node.NextSibling
	node.Parent.InsertBefore(newNode, nextSibling)
	if after != "" {
		node.Parent.InsertBefore(&html.Node{
			Type: html.TextNode,
			Data: after,
		}, nextSibling)
	}
}

func mentionProcessor(_ *postProcessCtx, node *html.Node) {
	m := mentionPattern.FindStringSubmatchIndex(node.Data)
	if m == nil {
		return
	}
	// Replace the mention with a link to the specified user.
	mention := node.Data[m[2]:m[3]]
	replaceContent(node, m[2], m[3], createLink(util.URLJoin(setting.AppURL, mention[1:]), mention))
}

func shortLinkProcessor(ctx *postProcessCtx, node *html.Node) {
	shortLinkProcessorFull(ctx, node, false)
}

func shortLinkProcessorFull(ctx *postProcessCtx, node *html.Node, noLink bool) {
	m := shortLinkPattern.FindStringSubmatchIndex(node.Data)
	if m == nil {
		return
	}

	content := node.Data[m[2]:m[3]]
	tail := node.Data[m[4]:m[5]]
	props := make(map[string]string)

	// MediaWiki uses [[link|text]], while GitHub uses [[text|link]]
	// It makes page handling terrible, but we prefer GitHub syntax
	// And fall back to MediaWiki only when it is obvious from the look
	// Of text and link contents
	sl := strings.Split(content, "|")
	for _, v := range sl {
		if equalPos := strings.IndexByte(v, '='); equalPos == -1 {
			// There is no equal in this argument; this is a mandatory arg
			if props["name"] == "" {
				if isLinkStr(v) {
					// If we clearly see it is a link, we save it so

					// But first we need to ensure, that if both mandatory args provided
					// look like links, we stick to GitHub syntax
					if props["link"] != "" {
						props["name"] = props["link"]
					}

					props["link"] = strings.TrimSpace(v)
				} else {
					props["name"] = v
				}
			} else {
				props["link"] = strings.TrimSpace(v)
			}
		} else {
			// There is an equal; optional argument.

			sep := strings.IndexByte(v, '=')
			key, val := v[:sep], html.UnescapeString(v[sep+1:])

			// When parsing HTML, x/net/html will change all quotes which are
			// not used for syntax into UTF-8 quotes. So checking val[0] won't
			// be enough, since that only checks a single byte.
			if (strings.HasPrefix(val, "“") && strings.HasSuffix(val, "”")) ||
				(strings.HasPrefix(val, "‘") && strings.HasSuffix(val, "’")) {
				const lenQuote = len("‘")
				val = val[lenQuote : len(val)-lenQuote]
			}
			props[key] = val
		}
	}

	var name, link string
	if props["link"] != "" {
		link = props["link"]
	} else if props["name"] != "" {
		link = props["name"]
	}
	if props["title"] != "" {
		name = props["title"]
	} else if props["name"] != "" {
		name = props["name"]
	} else {
		name = link
	}

	name += tail
	image := false
	switch ext := filepath.Ext(string(link)); ext {
	// fast path: empty string, ignore
	case "":
		break
	case ".jpg", ".jpeg", ".png", ".tif", ".tiff", ".webp", ".gif", ".bmp", ".ico", ".svg":
		image = true
	}

	childNode := &html.Node{}
	linkNode := &html.Node{
		FirstChild: childNode,
		LastChild:  childNode,
		Type:       html.ElementNode,
		Data:       "a",
		DataAtom:   atom.A,
	}
	childNode.Parent = linkNode
	absoluteLink := isLinkStr(link)
	if !absoluteLink {
		if image {
			link = strings.Replace(link, " ", "+", -1)
		} else {
			link = strings.Replace(link, " ", "-", -1)
		}
	}
	urlPrefix := ctx.urlPrefix
	if image {
		if !absoluteLink {
			if IsSameDomain(urlPrefix) {
				urlPrefix = strings.Replace(urlPrefix, "/src/", "/raw/", 1)
			}
			if ctx.isWikiMarkdown {
				link = util.URLJoin("wiki", "raw", link)
			}
			link = util.URLJoin(urlPrefix, link)
		}
		title := props["title"]
		if title == "" {
			title = props["alt"]
		}
		if title == "" {
			title = path.Base(string(name))
		}
		alt := props["alt"]
		if alt == "" {
			alt = name
		}

		// make the childNode an image - if we can, we also place the alt
		childNode.Type = html.ElementNode
		childNode.Data = "img"
		childNode.DataAtom = atom.Img
		childNode.Attr = []html.Attribute{
			{Key: "src", Val: link},
			{Key: "title", Val: title},
			{Key: "alt", Val: alt},
		}
		if alt == "" {
			childNode.Attr = childNode.Attr[:2]
		}
	} else {
		if !absoluteLink {
			if ctx.isWikiMarkdown {
				link = util.URLJoin("wiki", link)
			}
			link = util.URLJoin(urlPrefix, link)
		}
		childNode.Type = html.TextNode
		childNode.Data = name
	}
	if noLink {
		linkNode = childNode
	} else {
		linkNode.Attr = []html.Attribute{{Key: "href", Val: link}}
	}
	replaceContent(node, m[0], m[1], linkNode)
}

func fullIssuePatternProcessor(ctx *postProcessCtx, node *html.Node) {
	m := getIssueFullPattern().FindStringSubmatchIndex(node.Data)
	if m == nil {
		return
	}
	link := node.Data[m[0]:m[1]]
	id := "#" + node.Data[m[2]:m[3]]
	// TODO if m[4]:m[5] is not nil, then link is to a comment,
	// and we should indicate that in the text somehow
	replaceContent(node, m[0], m[1], createLink(link, id))
}

func issueIndexPatternProcessor(ctx *postProcessCtx, node *html.Node) {
	prefix := cutoutVerbosePrefix(ctx.urlPrefix)

	// default to numeric pattern, unless alphanumeric is requested.
	pattern := issueNumericPattern
	if ctx.metas["style"] == IssueNameStyleAlphanumeric {
		pattern = issueAlphanumericPattern
	}

	match := pattern.FindStringSubmatchIndex(node.Data)
	if match == nil {
		return
	}
	id := node.Data[match[2]:match[3]]
	var link *html.Node
	if ctx.metas == nil {
		link = createLink(util.URLJoin(prefix, "issues", id[1:]), id)
	} else {
		// Support for external issue tracker
		if ctx.metas["style"] == IssueNameStyleAlphanumeric {
			ctx.metas["index"] = id
		} else {
			ctx.metas["index"] = id[1:]
		}
		link = createLink(com.Expand(ctx.metas["format"], ctx.metas), id)
	}
	replaceContent(node, match[2], match[3], link)
}

func crossReferenceIssueIndexPatternProcessor(ctx *postProcessCtx, node *html.Node) {
	m := crossReferenceIssueNumericPattern.FindStringSubmatchIndex(node.Data)
	if m == nil {
		return
	}
	ref := node.Data[m[2]:m[3]]

	parts := strings.SplitN(ref, "#", 2)
	repo, issue := parts[0], parts[1]

	replaceContent(node, m[2], m[3],
		createLink(util.URLJoin(setting.AppURL, repo, "issues", issue), ref))
}

// fullSha1PatternProcessor renders SHA containing URLs
func fullSha1PatternProcessor(ctx *postProcessCtx, node *html.Node) {
	m := anySHA1Pattern.FindStringSubmatchIndex(node.Data)
	if m == nil {
		return
	}
	// take out what's relevant
	urlFull := node.Data[m[0]:m[1]]
	hash := node.Data[m[2]:m[3]]

	var subtree, line string

	// optional, we do them depending on the length.
	if m[7] > 0 {
		line = node.Data[m[6]:m[7]]
	}
	if m[5] > 0 {
		subtree = node.Data[m[4]:m[5]]
	}

	text := base.ShortSha(hash)
	if subtree != "" {
		text += "/" + subtree
	}
	if line != "" {
		text += " ("
		text += line
		text += ")"
	}

	replaceContent(node, m[0], m[1], createLink(urlFull, text))
}

// sha1CurrentPatternProcessor renders SHA1 strings to corresponding links that
// are assumed to be in the same repository.
func sha1CurrentPatternProcessor(ctx *postProcessCtx, node *html.Node) {
	m := sha1CurrentPattern.FindStringSubmatchIndex(node.Data)
	if m == nil {
		return
	}
	hash := node.Data[m[2]:m[3]]
	// The regex does not lie, it matches the hash pattern.
	// However, a regex cannot know if a hash actually exists or not.
	// We could assume that a SHA1 hash should probably contain alphas AND numerics
	// but that is not always the case.
	// Although unlikely, deadbeef and 1234567 are valid short forms of SHA1 hash
	// as used by git and github for linking and thus we have to do similar.
	replaceContent(node, m[2], m[3],
		createLink(util.URLJoin(ctx.urlPrefix, "commit", hash), base.ShortSha(hash)))
}

// emailAddressProcessor replaces raw email addresses with a mailto: link.
func emailAddressProcessor(ctx *postProcessCtx, node *html.Node) {
	m := emailRegex.FindStringIndex(node.Data)
	if m == nil {
		return
	}
	mail := node.Data[m[0]:m[1]]
	replaceContent(node, m[0], m[1], createLink("mailto:"+mail, mail))
}

// linkProcessor creates links for any HTTP or HTTPS URL not captured by
// markdown.
func linkProcessor(ctx *postProcessCtx, node *html.Node) {
	m := linkRegex.FindStringIndex(node.Data)
	if m == nil {
		return
	}
	uri := node.Data[m[0]:m[1]]
	replaceContent(node, m[0], m[1], createLink(uri, uri))
}

func genDefaultLinkProcessor(defaultLink string) processor {
	return func(ctx *postProcessCtx, node *html.Node) {
		ch := &html.Node{
			Parent: node,
			Type:   html.TextNode,
			Data:   node.Data,
		}

		node.Type = html.ElementNode
		node.Data = "a"
		node.DataAtom = atom.A
		node.Attr = []html.Attribute{{Key: "href", Val: defaultLink}}
		node.FirstChild, node.LastChild = ch, ch
	}
}
