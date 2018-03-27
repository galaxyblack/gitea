// Copyright 2017 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	api "code.gitea.io/sdk/gitea"
)

// NewCommitStatus creates a new CommitStatus
func NewCommitStatus(ctx *context.APIContext, form api.CreateStatusOption) {
	// swagger:operation POST /repos/{owner}/{repo}/statuses/{sha} repository repoCreateStatus
	// ---
	// summary: Create a commit status
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
	// - name: sha
	//   in: path
	//   description: sha of the commit
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateStatusOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/StatusList"
	sha := ctx.Params("sha")
	if len(sha) == 0 {
		sha = ctx.Params("ref")
	}
	if len(sha) == 0 {
		ctx.Error(400, "ref/sha not given", nil)
		return
	}
	status := &models.CommitStatus{
		State:       models.CommitStatusState(form.State),
		TargetURL:   form.TargetURL,
		Description: form.Description,
		Context:     form.Context,
	}
	if err := models.NewCommitStatus(ctx.Repo.Repository, ctx.User, sha, status); err != nil {
		ctx.Error(500, "NewCommitStatus", err)
		return
	}

	newStatus, err := models.GetCommitStatus(ctx.Repo.Repository, sha, status)
	if err != nil {
		ctx.Error(500, "GetCommitStatus", err)
		return
	}
	ctx.JSON(201, newStatus.APIFormat())
}

// GetCommitStatuses returns all statuses for any given commit hash
func GetCommitStatuses(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/statuses/{sha} repository repoListStatuses
	// ---
	// summary: Get a commit's statuses
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
	// - name: sha
	//   in: path
	//   description: sha of the commit
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/StatusList"
	getCommitStatuses(ctx, ctx.Params("sha"))
}

// GetCommitStatusesByRef returns all statuses for any given commit ref
func GetCommitStatusesByRef(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/commits/{ref}/statuses repository repoListStatusesByRef
	// ---
	// summary: Get a commit's statuses, by branch/tag/commit reference
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
	// - name: ref
	//   in: path
	//   description: name of branch/tag/commit
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/StatusList"
	getCommitStatuses(ctx, ctx.Params("ref"))
}

func getCommitStatuses(ctx *context.APIContext, sha string) {
	if len(sha) == 0 {
		ctx.Error(400, "ref/sha not given", nil)
		return
	}
	repo := ctx.Repo.Repository

	page := ctx.ParamsInt("page")

	statuses, err := models.GetCommitStatuses(repo, sha, page)
	if err != nil {
		ctx.Error(500, "GetCommitStatuses", fmt.Errorf("GetCommitStatuses[%s, %s, %d]: %v", repo.FullName(), sha, page, err))
	}

	apiStatuses := make([]*api.Status, 0, len(statuses))
	for _, status := range statuses {
		apiStatuses = append(apiStatuses, status.APIFormat())
	}

	ctx.JSON(200, apiStatuses)
}

type combinedCommitStatus struct {
	State      models.CommitStatusState `json:"state"`
	SHA        string                   `json:"sha"`
	TotalCount int                      `json:"total_count"`
	Statuses   []*api.Status            `json:"statuses"`
	Repo       *api.Repository          `json:"repository"`
	CommitURL  string                   `json:"commit_url"`
	URL        string                   `json:"url"`
}

// GetCombinedCommitStatusByRef returns the combined status for any given commit hash
func GetCombinedCommitStatusByRef(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/commits/{ref}/statuses repository repoGetCombinedStatusByRef
	// ---
	// summary: Get a commit's combined status, by branch/tag/commit reference
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
	// - name: ref
	//   in: path
	//   description: name of branch/tag/commit
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Status"
	sha := ctx.Params("ref")
	if len(sha) == 0 {
		ctx.Error(400, "ref/sha not given", nil)
		return
	}
	repo := ctx.Repo.Repository

	page := ctx.ParamsInt("page")

	statuses, err := models.GetLatestCommitStatus(repo, sha, page)
	if err != nil {
		ctx.Error(500, "GetLatestCommitStatus", fmt.Errorf("GetLatestCommitStatus[%s, %s, %d]: %v", repo.FullName(), sha, page, err))
		return
	}

	if len(statuses) == 0 {
		ctx.Status(200)
		return
	}

	retStatus := &combinedCommitStatus{
		SHA:        sha,
		TotalCount: len(statuses),
		Repo:       repo.APIFormat(ctx.Repo.AccessMode),
		URL:        "",
	}

	retStatus.Statuses = make([]*api.Status, 0, len(statuses))
	for _, status := range statuses {
		retStatus.Statuses = append(retStatus.Statuses, status.APIFormat())
		if status.State.IsWorseThan(retStatus.State) {
			retStatus.State = status.State
		}
	}

	ctx.JSON(200, retStatus)
}
