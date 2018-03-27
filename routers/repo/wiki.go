// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"code.gitea.io/git"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/util"
)

const (
	tplWikiStart base.TplName = "repo/wiki/start"
	tplWikiView  base.TplName = "repo/wiki/view"
	tplWikiNew   base.TplName = "repo/wiki/new"
	tplWikiPages base.TplName = "repo/wiki/pages"
)

// MustEnableWiki check if wiki is enabled, if external then redirect
func MustEnableWiki(ctx *context.Context) {
	if !ctx.Repo.Repository.UnitEnabled(models.UnitTypeWiki) &&
		!ctx.Repo.Repository.UnitEnabled(models.UnitTypeExternalWiki) {
		ctx.NotFound("MustEnableWiki", nil)
		return
	}

	unit, err := ctx.Repo.Repository.GetUnit(models.UnitTypeExternalWiki)
	if err == nil {
		ctx.Redirect(unit.ExternalWikiConfig().ExternalWikiURL)
		return
	}
}

// PageMeta wiki page meat information
type PageMeta struct {
	Name        string
	SubURL      string
	UpdatedUnix util.TimeStamp
}

// findEntryForFile finds the tree entry for a target filepath.
func findEntryForFile(commit *git.Commit, target string) (*git.TreeEntry, error) {
	entries, err := commit.ListEntries()
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.Type == git.ObjectBlob && entry.Name() == target {
			return entry, nil
		}
	}
	return nil, nil
}

func findWikiRepoCommit(ctx *context.Context) (*git.Repository, *git.Commit, error) {
	wikiRepo, err := git.OpenRepository(ctx.Repo.Repository.WikiPath())
	if err != nil {
		ctx.ServerError("OpenRepository", err)
		return nil, nil, err
	}

	commit, err := wikiRepo.GetBranchCommit("master")
	if err != nil {
		ctx.ServerError("GetBranchCommit", err)
		return wikiRepo, nil, err
	}
	return wikiRepo, commit, nil
}

// wikiContentsByEntry returns the contents of the wiki page referenced by the
// given tree entry. Writes to ctx if an error occurs.
func wikiContentsByEntry(ctx *context.Context, entry *git.TreeEntry) []byte {
	reader, err := entry.Blob().Data()
	if err != nil {
		ctx.ServerError("Blob.Data", err)
		return nil
	}
	content, err := ioutil.ReadAll(reader)
	if err != nil {
		ctx.ServerError("ReadAll", err)
		return nil
	}
	return content
}

// wikiContentsByName returns the contents of a wiki page, along with a boolean
// indicating whether the page exists. Writes to ctx if an error occurs.
func wikiContentsByName(ctx *context.Context, commit *git.Commit, wikiName string) ([]byte, bool) {
	entry, err := findEntryForFile(commit, models.WikiNameToFilename(wikiName))
	if err != nil {
		ctx.ServerError("findEntryForFile", err)
		return nil, false
	} else if entry == nil {
		return nil, false
	}
	return wikiContentsByEntry(ctx, entry), true
}

func renderWikiPage(ctx *context.Context, isViewPage bool) (*git.Repository, *git.TreeEntry) {
	wikiRepo, commit, err := findWikiRepoCommit(ctx)
	if err != nil {
		return nil, nil
	}

	// Get page list.
	if isViewPage {
		entries, err := commit.ListEntries()
		if err != nil {
			ctx.ServerError("ListEntries", err)
			return nil, nil
		}
		pages := make([]PageMeta, 0, len(entries))
		for _, entry := range entries {
			if entry.Type != git.ObjectBlob {
				continue
			}
			wikiName, err := models.WikiFilenameToName(entry.Name())
			if err != nil {
				if models.IsErrWikiInvalidFileName(err) {
					continue
				}
				ctx.ServerError("WikiFilenameToName", err)
				return nil, nil
			} else if wikiName == "_Sidebar" || wikiName == "_Footer" {
				continue
			}
			pages = append(pages, PageMeta{
				Name:   wikiName,
				SubURL: models.WikiNameToSubURL(wikiName),
			})
		}
		ctx.Data["Pages"] = pages
	}

	pageName := models.NormalizeWikiName(ctx.Params(":page"))
	if len(pageName) == 0 {
		pageName = "Home"
	}
	ctx.Data["PageURL"] = models.WikiNameToSubURL(pageName)

	ctx.Data["old_title"] = pageName
	ctx.Data["Title"] = pageName
	ctx.Data["title"] = pageName
	ctx.Data["RequireHighlightJS"] = true

	pageFilename := models.WikiNameToFilename(pageName)
	var entry *git.TreeEntry
	if entry, err = findEntryForFile(commit, pageFilename); err != nil {
		ctx.ServerError("findEntryForFile", err)
		return nil, nil
	} else if entry == nil {
		ctx.Redirect(ctx.Repo.RepoLink + "/wiki/_pages")
		return nil, nil
	}
	data := wikiContentsByEntry(ctx, entry)
	if ctx.Written() {
		return nil, nil
	}

	if isViewPage {
		sidebarContent, sidebarPresent := wikiContentsByName(ctx, commit, "_Sidebar")
		if ctx.Written() {
			return nil, nil
		}

		footerContent, footerPresent := wikiContentsByName(ctx, commit, "_Footer")
		if ctx.Written() {
			return nil, nil
		}

		metas := ctx.Repo.Repository.ComposeMetas()
		ctx.Data["content"] = markdown.RenderWiki(data, ctx.Repo.RepoLink, metas)
		ctx.Data["sidebarPresent"] = sidebarPresent
		ctx.Data["sidebarContent"] = markdown.RenderWiki(sidebarContent, ctx.Repo.RepoLink, metas)
		ctx.Data["footerPresent"] = footerPresent
		ctx.Data["footerContent"] = markdown.RenderWiki(footerContent, ctx.Repo.RepoLink, metas)
	} else {
		ctx.Data["content"] = string(data)
		ctx.Data["sidebarPresent"] = false
		ctx.Data["sidebarContent"] = ""
		ctx.Data["footerPresent"] = false
		ctx.Data["footerContent"] = ""
	}

	return wikiRepo, entry
}

// Wiki renders single wiki page
func Wiki(ctx *context.Context) {
	ctx.Data["PageIsWiki"] = true

	if !ctx.Repo.Repository.HasWiki() {
		ctx.Data["Title"] = ctx.Tr("repo.wiki")
		ctx.HTML(200, tplWikiStart)
		return
	}

	wikiRepo, entry := renderWikiPage(ctx, true)
	if ctx.Written() {
		return
	}
	if entry == nil {
		ctx.Data["Title"] = ctx.Tr("repo.wiki")
		ctx.HTML(200, tplWikiStart)
		return
	}

	wikiPath := entry.Name()
	if markup.Type(wikiPath) != markdown.MarkupName {
		ext := strings.ToUpper(filepath.Ext(wikiPath))
		ctx.Data["FormatWarning"] = fmt.Sprintf("%s rendering is not supported at the moment. Rendered as Markdown.", ext)
	}
	// Get last change information.
	lastCommit, err := wikiRepo.GetCommitByPath(wikiPath)
	if err != nil {
		ctx.ServerError("GetCommitByPath", err)
		return
	}
	ctx.Data["Author"] = lastCommit.Author

	ctx.HTML(200, tplWikiView)
}

// WikiPages render wiki pages list page
func WikiPages(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.wiki.pages")
	ctx.Data["PageIsWiki"] = true

	if !ctx.Repo.Repository.HasWiki() {
		ctx.Redirect(ctx.Repo.RepoLink + "/wiki")
		return
	}

	wikiRepo, commit, err := findWikiRepoCommit(ctx)
	if err != nil {
		return
	}

	entries, err := commit.ListEntries()
	if err != nil {
		ctx.ServerError("ListEntries", err)
		return
	}
	pages := make([]PageMeta, 0, len(entries))
	for _, entry := range entries {
		if entry.Type != git.ObjectBlob {
			continue
		}
		c, err := wikiRepo.GetCommitByPath(entry.Name())
		if err != nil {
			ctx.ServerError("GetCommit", err)
			return
		}
		wikiName, err := models.WikiFilenameToName(entry.Name())
		if err != nil {
			if models.IsErrWikiInvalidFileName(err) {
				continue
			}
			ctx.ServerError("WikiFilenameToName", err)
			return
		}
		pages = append(pages, PageMeta{
			Name:        wikiName,
			SubURL:      models.WikiNameToSubURL(wikiName),
			UpdatedUnix: util.TimeStamp(c.Author.When.Unix()),
		})
	}
	ctx.Data["Pages"] = pages

	ctx.HTML(200, tplWikiPages)
}

// WikiRaw outputs raw blob requested by user (image for example)
func WikiRaw(ctx *context.Context) {
	wikiRepo, commit, err := findWikiRepoCommit(ctx)
	if err != nil {
		if wikiRepo != nil {
			return
		}
	}
	providedPath := ctx.Params("*")
	if strings.HasSuffix(providedPath, ".md") {
		providedPath = providedPath[:len(providedPath)-3]
	}
	wikiPath := models.WikiNameToFilename(providedPath)
	var entry *git.TreeEntry
	if commit != nil {
		entry, err = findEntryForFile(commit, wikiPath)
	}
	if err != nil {
		ctx.ServerError("findFile", err)
		return
	} else if entry == nil {
		ctx.NotFound("findEntryForFile", nil)
		return
	}

	if err = ServeBlob(ctx, entry.Blob()); err != nil {
		ctx.ServerError("ServeBlob", err)
	}
}

// NewWiki render wiki create page
func NewWiki(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.wiki.new_page")
	ctx.Data["PageIsWiki"] = true
	ctx.Data["RequireSimpleMDE"] = true

	if !ctx.Repo.Repository.HasWiki() {
		ctx.Data["title"] = "Home"
	}

	ctx.HTML(200, tplWikiNew)
}

// NewWikiPost response for wiki create request
func NewWikiPost(ctx *context.Context, form auth.NewWikiForm) {
	ctx.Data["Title"] = ctx.Tr("repo.wiki.new_page")
	ctx.Data["PageIsWiki"] = true
	ctx.Data["RequireSimpleMDE"] = true

	if ctx.HasError() {
		ctx.HTML(200, tplWikiNew)
		return
	}

	wikiName := models.NormalizeWikiName(form.Title)
	if err := ctx.Repo.Repository.AddWikiPage(ctx.User, wikiName, form.Content, form.Message); err != nil {
		if models.IsErrWikiReservedName(err) {
			ctx.Data["Err_Title"] = true
			ctx.RenderWithErr(ctx.Tr("repo.wiki.reserved_page", wikiName), tplWikiNew, &form)
		} else if models.IsErrWikiAlreadyExist(err) {
			ctx.Data["Err_Title"] = true
			ctx.RenderWithErr(ctx.Tr("repo.wiki.page_already_exists"), tplWikiNew, &form)
		} else {
			ctx.ServerError("AddWikiPage", err)
		}
		return
	}

	ctx.Redirect(ctx.Repo.RepoLink + "/wiki/" + models.WikiNameToFilename(wikiName))
}

// EditWiki render wiki modify page
func EditWiki(ctx *context.Context) {
	ctx.Data["PageIsWiki"] = true
	ctx.Data["PageIsWikiEdit"] = true
	ctx.Data["RequireSimpleMDE"] = true

	if !ctx.Repo.Repository.HasWiki() {
		ctx.Redirect(ctx.Repo.RepoLink + "/wiki")
		return
	}

	renderWikiPage(ctx, false)
	if ctx.Written() {
		return
	}

	ctx.HTML(200, tplWikiNew)
}

// EditWikiPost response for wiki modify request
func EditWikiPost(ctx *context.Context, form auth.NewWikiForm) {
	ctx.Data["Title"] = ctx.Tr("repo.wiki.new_page")
	ctx.Data["PageIsWiki"] = true
	ctx.Data["RequireSimpleMDE"] = true

	if ctx.HasError() {
		ctx.HTML(200, tplWikiNew)
		return
	}

	oldWikiName := models.NormalizeWikiName(ctx.Params(":page"))
	newWikiName := models.NormalizeWikiName(form.Title)

	if err := ctx.Repo.Repository.EditWikiPage(ctx.User, oldWikiName, newWikiName, form.Content, form.Message); err != nil {
		ctx.ServerError("EditWikiPage", err)
		return
	}

	ctx.Redirect(ctx.Repo.RepoLink + "/wiki/" + models.WikiNameToFilename(newWikiName))
}

// DeleteWikiPagePost delete wiki page
func DeleteWikiPagePost(ctx *context.Context) {
	wikiName := models.NormalizeWikiName(ctx.Params(":page"))
	if len(wikiName) == 0 {
		wikiName = "Home"
	}

	if err := ctx.Repo.Repository.DeleteWikiPage(ctx.User, wikiName); err != nil {
		ctx.ServerError("DeleteWikiPage", err)
		return
	}

	ctx.JSON(200, map[string]interface{}{
		"redirect": ctx.Repo.RepoLink + "/wiki/",
	})
}
