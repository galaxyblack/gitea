// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	api "code.gitea.io/sdk/gitea"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
)

// GetRelease get a single release of a repository
func GetRelease(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/releases/{id} repository repoGetRelease
	// ---
	// summary: Get a release
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the release to get
	//   type: integer
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Release"
	id := ctx.ParamsInt64(":id")
	release, err := models.GetReleaseByID(id)
	if err != nil {
		ctx.Error(500, "GetReleaseByID", err)
		return
	}
	if release.RepoID != ctx.Repo.Repository.ID {
		ctx.Status(404)
		return
	}
	if err := release.LoadAttributes(); err != nil {
		ctx.Error(500, "LoadAttributes", err)
		return
	}
	ctx.JSON(200, release.APIFormat())
}

// ListReleases list a repository's releases
func ListReleases(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/releases repository repoListReleases
	// ---
	// summary: List a repo's releases
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/ReleaseList"
	releases, err := models.GetReleasesByRepoID(ctx.Repo.Repository.ID, models.FindReleasesOptions{
		IncludeDrafts: ctx.Repo.AccessMode >= models.AccessModeWrite,
		IncludeTags:   false,
	}, 1, 2147483647)
	if err != nil {
		ctx.Error(500, "GetReleasesByRepoID", err)
		return
	}
	rels := make([]*api.Release, len(releases))
	for i, release := range releases {
		if err := release.LoadAttributes(); err != nil {
			ctx.Error(500, "LoadAttributes", err)
			return
		}
		rels[i] = release.APIFormat()
	}
	ctx.JSON(200, rels)
}

// CreateRelease create a release
func CreateRelease(ctx *context.APIContext, form api.CreateReleaseOption) {
	// swagger:operation POST /repos/{owner}/{repo}/releases repository repoCreateRelease
	// ---
	// summary: Create a release
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateReleaseOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Release"
	if ctx.Repo.AccessMode < models.AccessModeWrite {
		ctx.Status(403)
		return
	}
	if !ctx.Repo.GitRepo.IsTagExist(form.TagName) {
		ctx.Status(404)
		return
	}
	rel, err := models.GetRelease(ctx.Repo.Repository.ID, form.TagName)
	if err != nil {
		if !models.IsErrReleaseNotExist(err) {
			ctx.ServerError("GetRelease", err)
			return
		}
		rel = &models.Release{
			RepoID:       ctx.Repo.Repository.ID,
			PublisherID:  ctx.User.ID,
			Publisher:    ctx.User,
			TagName:      form.TagName,
			Target:       form.Target,
			Title:        form.Title,
			Note:         form.Note,
			IsDraft:      form.IsDraft,
			IsPrerelease: form.IsPrerelease,
			IsTag:        false,
			Repo:         ctx.Repo.Repository,
		}
		if err := models.CreateRelease(ctx.Repo.GitRepo, rel, nil); err != nil {
			if models.IsErrReleaseAlreadyExist(err) {
				ctx.Status(409)
			} else {
				ctx.Error(500, "CreateRelease", err)
			}
			return
		}
	} else {
		if !rel.IsTag {
			ctx.Status(409)
			return
		}

		rel.Title = form.Title
		rel.Note = form.Note
		rel.IsDraft = form.IsDraft
		rel.IsPrerelease = form.IsPrerelease
		rel.PublisherID = ctx.User.ID
		rel.IsTag = false
		rel.Repo = ctx.Repo.Repository
		rel.Publisher = ctx.User

		if err = models.UpdateRelease(ctx.Repo.GitRepo, rel, nil); err != nil {
			ctx.ServerError("UpdateRelease", err)
			return
		}
	}
	ctx.JSON(201, rel.APIFormat())
}

// EditRelease edit a release
func EditRelease(ctx *context.APIContext, form api.EditReleaseOption) {
	// swagger:operation PATCH /repos/{owner}/{repo}/releases/{id} repository repoEditRelease
	// ---
	// summary: Update a release
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the release to edit
	//   type: integer
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditReleaseOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/Release"
	if ctx.Repo.AccessMode < models.AccessModeWrite {
		ctx.Status(403)
		return
	}
	id := ctx.ParamsInt64(":id")
	rel, err := models.GetReleaseByID(id)
	if err != nil && !models.IsErrReleaseNotExist(err) {
		ctx.Error(500, "GetReleaseByID", err)
		return
	}
	if err != nil && models.IsErrReleaseNotExist(err) ||
		rel.IsTag || rel.RepoID != ctx.Repo.Repository.ID {
		ctx.Status(404)
		return
	}

	if len(form.TagName) > 0 {
		rel.TagName = form.TagName
	}
	if len(form.Target) > 0 {
		rel.Target = form.Target
	}
	if len(form.Title) > 0 {
		rel.Title = form.Title
	}
	if len(form.Note) > 0 {
		rel.Note = form.Note
	}
	if form.IsDraft != nil {
		rel.IsDraft = *form.IsDraft
	}
	if form.IsPrerelease != nil {
		rel.IsPrerelease = *form.IsPrerelease
	}
	if err := models.UpdateRelease(ctx.Repo.GitRepo, rel, nil); err != nil {
		ctx.Error(500, "UpdateRelease", err)
		return
	}

	rel, err = models.GetReleaseByID(id)
	if err != nil {
		ctx.Error(500, "GetReleaseByID", err)
		return
	}
	if err := rel.LoadAttributes(); err != nil {
		ctx.Error(500, "LoadAttributes", err)
		return
	}
	ctx.JSON(200, rel.APIFormat())
}

// DeleteRelease delete a release from a repository
func DeleteRelease(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/releases/{id} repository repoDeleteRelease
	// ---
	// summary: Delete a release
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the release to delete
	//   type: integer
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	if ctx.Repo.AccessMode < models.AccessModeWrite {
		ctx.Status(403)
		return
	}
	id := ctx.ParamsInt64(":id")
	rel, err := models.GetReleaseByID(id)
	if err != nil && !models.IsErrReleaseNotExist(err) {
		ctx.Error(500, "GetReleaseByID", err)
		return
	}
	if err != nil && models.IsErrReleaseNotExist(err) ||
		rel.IsTag || rel.RepoID != ctx.Repo.Repository.ID {
		ctx.Status(404)
		return
	}
	if err := models.DeleteReleaseByID(id, ctx.User, false); err != nil {
		ctx.Error(500, "DeleteReleaseByID", err)
		return
	}
	ctx.Status(204)
}
