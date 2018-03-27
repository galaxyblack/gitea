// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/sdk/gitea"
	"errors"
	"net/http"
	"strings"
)

// GetReleaseAttachment gets a single attachment of the release
func GetReleaseAttachment(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/releases/{id}/assets/{attachment_id} repository repoGetReleaseAttachment
	// ---
	// summary: Get a release attachment
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
	//   description: id of the release
	//   type: integer
	//   required: true
	// - name: attachment_id
	//   in: path
	//   description: id of the attachment to get
	//   type: integer
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Attachment"
	releaseID := ctx.ParamsInt64(":id")
	attachID := ctx.ParamsInt64(":asset")
	attach, err := models.GetAttachmentByID(attachID)
	if err != nil {
		ctx.Error(500, "GetAttachmentByID", err)
		return
	}
	if attach.ReleaseID != releaseID {
		ctx.Status(404)
		return
	}
	// FIXME Should prove the existence of the given repo, but results in unnecessary database requests
	ctx.JSON(200, attach.APIFormat())
}

// ListReleaseAttachments lists all attachments of the release
func ListReleaseAttachments(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/releases/{id}/assets repository repoListReleaseAttachments
	// ---
	// summary: List release's attachments
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
	//   description: id of the release
	//   type: integer
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/AttachmentList"
	releaseID := ctx.ParamsInt64(":id")
	release, err := models.GetReleaseByID(releaseID)
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
	ctx.JSON(200, release.APIFormat().Attachments)
}

// CreateReleaseAttachment creates an attachment and saves the given file
func CreateReleaseAttachment(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/releases/{id}/assets repository repoCreateReleaseAttachment
	// ---
	// summary: Create a release attachment
	// produces:
	// - application/json
	// consumes:
	// - multipart/form-data
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
	//   description: id of the release
	//   type: integer
	//   required: true
	// - name: name
	//   in: query
	//   description: name of the attachment
	//   type: string
	//   required: false
	// - name: attachment
	//   in: formData
	//   description: attachment to upload
	//   type: file
	//   required: true
	// responses:
	//   "201":
	//     "$ref": "#/responses/Attachment"

	// Check if attachments are enabled
	if !setting.AttachmentEnabled {
		ctx.Error(404, "AttachmentEnabled", errors.New("attachment is not enabled"))
		return
	}

	// Check if release exists an load release
	releaseID := ctx.ParamsInt64(":id")
	release, err := models.GetReleaseByID(releaseID)
	if err != nil {
		ctx.Error(500, "GetReleaseByID", err)
		return
	}

	// Get uploaded file from request
	file, header, err := ctx.GetFile("attachment")
	if err != nil {
		ctx.Error(500, "GetFile", err)
		return
	}
	defer file.Close()

	buf := make([]byte, 1024)
	n, _ := file.Read(buf)
	if n > 0 {
		buf = buf[:n]
	}

	// Check if the filetype is allowed by the settings
	fileType := http.DetectContentType(buf)

	allowedTypes := strings.Split(setting.AttachmentAllowedTypes, ",")
	allowed := false
	for _, t := range allowedTypes {
		t := strings.Trim(t, " ")
		if t == "*/*" || t == fileType {
			allowed = true
			break
		}
	}

	if !allowed {
		ctx.Error(400, "DetectContentType", errors.New("File type is not allowed"))
		return
	}

	var filename = header.Filename
	if query := ctx.Query("name"); query != "" {
		filename = query
	}

	// Create a new attachment and save the file
	attach, err := models.NewAttachment(filename, buf, file)
	if err != nil {
		ctx.Error(500, "NewAttachment", err)
		return
	}
	attach.ReleaseID = release.ID
	if err := models.UpdateAttachment(attach); err != nil {
		ctx.Error(500, "UpdateAttachment", err)
		return
	}
	ctx.JSON(201, attach.APIFormat())
}

// EditReleaseAttachment updates the given attachment
func EditReleaseAttachment(ctx *context.APIContext, form api.EditAttachmentOptions) {
	// swagger:operation PATCH /repos/{owner}/{repo}/releases/{id}/assets/{attachment_id} repository repoEditReleaseAttachment
	// ---
	// summary: Edit a release attachment
	// produces:
	// - application/json
	// consumes:
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
	//   description: id of the release
	//   type: integer
	//   required: true
	// - name: attachment_id
	//   in: path
	//   description: id of the attachment to edit
	//   type: integer
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditAttachmentOptions"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Attachment"

	// Check if release exists an load release
	releaseID := ctx.ParamsInt64(":id")
	attachID := ctx.ParamsInt64(":attachment")
	attach, err := models.GetAttachmentByID(attachID)
	if err != nil {
		ctx.Error(500, "GetAttachmentByID", err)
		return
	}
	if attach.ReleaseID != releaseID {
		ctx.Status(404)
		return
	}
	// FIXME Should prove the existence of the given repo, but results in unnecessary database requests
	if form.Name != "" {
		attach.Name = form.Name
	}

	if err := models.UpdateAttachment(attach); err != nil {
		ctx.Error(500, "UpdateAttachment", attach)
	}
	ctx.JSON(201, attach.APIFormat())
}

// DeleteReleaseAttachment delete a given attachment
func DeleteReleaseAttachment(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/releases/{id}/assets/{attachment_id} repository repoDeleteReleaseAttachment
	// ---
	// summary: Delete a release attachment
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
	//   description: id of the release
	//   type: integer
	//   required: true
	// - name: attachment_id
	//   in: path
	//   description: id of the attachment to delete
	//   type: integer
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"

	// Check if release exists an load release
	releaseID := ctx.ParamsInt64(":id")
	attachID := ctx.ParamsInt64(":attachment")
	attach, err := models.GetAttachmentByID(attachID)
	if err != nil {
		ctx.Error(500, "GetAttachmentByID", err)
		return
	}
	if attach.ReleaseID != releaseID {
		ctx.Status(404)
		return
	}
	// FIXME Should prove the existence of the given repo, but results in unnecessary database requests

	if err := models.DeleteAttachment(attach, true); err != nil {
		ctx.Error(500, "DeleteAttachment", err)
		return
	}
	ctx.Status(204)
}
