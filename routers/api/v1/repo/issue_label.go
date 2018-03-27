// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	api "code.gitea.io/sdk/gitea"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
)

// ListIssueLabels list all the labels of an issue
func ListIssueLabels(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/issues/{index}/labels issue issueGetLabels
	// ---
	// summary: Get an issue's labels
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
	// - name: index
	//   in: path
	//   description: index of the issue
	//   type: integer
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/LabelList"
	//   "404":
	//     "$ref": "#/responses/notFound"
	issue, err := models.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrIssueNotExist(err) {
			ctx.Status(404)
		} else {
			ctx.Error(500, "GetIssueByIndex", err)
		}
		return
	}

	apiLabels := make([]*api.Label, len(issue.Labels))
	for i := range issue.Labels {
		apiLabels[i] = issue.Labels[i].APIFormat()
	}
	ctx.JSON(200, &apiLabels)
}

// AddIssueLabels add labels for an issue
func AddIssueLabels(ctx *context.APIContext, form api.IssueLabelsOption) {
	// swagger:operation POST /repos/{owner}/{repo}/issue/{index}/labels issue issueAddLabel
	// ---
	// summary: Add a label to an issue
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
	// - name: index
	//   in: path
	//   description: index of the issue
	//   type: integer
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/IssueLabelsOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/LabelList"
	if !ctx.Repo.IsWriter() {
		ctx.Status(403)
		return
	}

	issue, err := models.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrIssueNotExist(err) {
			ctx.Status(404)
		} else {
			ctx.Error(500, "GetIssueByIndex", err)
		}
		return
	}

	labels, err := models.GetLabelsInRepoByIDs(ctx.Repo.Repository.ID, form.Labels)
	if err != nil {
		ctx.Error(500, "GetLabelsInRepoByIDs", err)
		return
	}

	if err = issue.AddLabels(ctx.User, labels); err != nil {
		ctx.Error(500, "AddLabels", err)
		return
	}

	labels, err = models.GetLabelsByIssueID(issue.ID)
	if err != nil {
		ctx.Error(500, "GetLabelsByIssueID", err)
		return
	}

	apiLabels := make([]*api.Label, len(labels))
	for i := range labels {
		apiLabels[i] = labels[i].APIFormat()
	}
	ctx.JSON(200, &apiLabels)
}

// DeleteIssueLabel delete a label for an issue
func DeleteIssueLabel(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/issue/{index}/labels/{id} issue issueRemoveLabel
	// ---
	// summary: Remove a label from an issue
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
	// - name: index
	//   in: path
	//   description: index of the issue
	//   type: integer
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the label to remove
	//   type: integer
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	if !ctx.Repo.IsWriter() {
		ctx.Status(403)
		return
	}

	issue, err := models.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrIssueNotExist(err) {
			ctx.Status(404)
		} else {
			ctx.Error(500, "GetIssueByIndex", err)
		}
		return
	}

	label, err := models.GetLabelInRepoByID(ctx.Repo.Repository.ID, ctx.ParamsInt64(":id"))
	if err != nil {
		if models.IsErrLabelNotExist(err) {
			ctx.Error(422, "", err)
		} else {
			ctx.Error(500, "GetLabelInRepoByID", err)
		}
		return
	}

	if err := models.DeleteIssueLabel(issue, label, ctx.User); err != nil {
		ctx.Error(500, "DeleteIssueLabel", err)
		return
	}

	ctx.Status(204)
}

// ReplaceIssueLabels replace labels for an issue
func ReplaceIssueLabels(ctx *context.APIContext, form api.IssueLabelsOption) {
	// swagger:operation PUT /repos/{owner}/{repo}/issue/{index}/labels issue issueReplaceLabels
	// ---
	// summary: Replace an issue's labels
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
	// - name: index
	//   in: path
	//   description: index of the issue
	//   type: integer
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/IssueLabelsOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/LabelList"
	if !ctx.Repo.IsWriter() {
		ctx.Status(403)
		return
	}

	issue, err := models.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrIssueNotExist(err) {
			ctx.Status(404)
		} else {
			ctx.Error(500, "GetIssueByIndex", err)
		}
		return
	}

	labels, err := models.GetLabelsInRepoByIDs(ctx.Repo.Repository.ID, form.Labels)
	if err != nil {
		ctx.Error(500, "GetLabelsInRepoByIDs", err)
		return
	}

	if err := issue.ReplaceLabels(labels, ctx.User); err != nil {
		ctx.Error(500, "ReplaceLabels", err)
		return
	}

	labels, err = models.GetLabelsByIssueID(issue.ID)
	if err != nil {
		ctx.Error(500, "GetLabelsByIssueID", err)
		return
	}

	apiLabels := make([]*api.Label, len(labels))
	for i := range labels {
		apiLabels[i] = labels[i].APIFormat()
	}
	ctx.JSON(200, &apiLabels)
}

// ClearIssueLabels delete all the labels for an issue
func ClearIssueLabels(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/issue/{index}/labels issue issueClearLabels
	// ---
	// summary: Remove all labels from an issue
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
	// - name: index
	//   in: path
	//   description: index of the issue
	//   type: integer
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	if !ctx.Repo.IsWriter() {
		ctx.Status(403)
		return
	}

	issue, err := models.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrIssueNotExist(err) {
			ctx.Status(404)
		} else {
			ctx.Error(500, "GetIssueByIndex", err)
		}
		return
	}

	if err := issue.ClearLabels(ctx.User); err != nil {
		ctx.Error(500, "ClearLabels", err)
		return
	}

	ctx.Status(204)
}
