// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package org

import (
	api "code.gitea.io/sdk/gitea"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/routers/api/v1/convert"
	"code.gitea.io/gitea/routers/api/v1/utils"
)

// ListHooks list an organziation's webhooks
func ListHooks(ctx *context.APIContext) {
	// swagger:operation GET /orgs/{org}/hooks organization orgListHooks
	// ---
	// summary: List an organization's webhooks
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/HookList"
	org := ctx.Org.Organization
	orgHooks, err := models.GetWebhooksByOrgID(org.ID)
	if err != nil {
		ctx.Error(500, "GetWebhooksByOrgID", err)
		return
	}
	hooks := make([]*api.Hook, len(orgHooks))
	for i, hook := range orgHooks {
		hooks[i] = convert.ToHook(org.HomeLink(), hook)
	}
	ctx.JSON(200, hooks)
}

// GetHook get an organization's hook by id
func GetHook(ctx *context.APIContext) {
	// swagger:operation GET /orgs/{org}/hooks/{id} organization orgGetHook
	// ---
	// summary: Get a hook
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/Hook"
	org := ctx.Org.Organization
	hookID := ctx.ParamsInt64(":id")
	hook, err := utils.GetOrgHook(ctx, org.ID, hookID)
	if err != nil {
		return
	}
	ctx.JSON(200, convert.ToHook(org.HomeLink(), hook))
}

// CreateHook create a hook for an organization
func CreateHook(ctx *context.APIContext, form api.CreateHookOption) {
	// swagger:operation POST /orgs/{org}/hooks/ organization orgCreateHook
	// ---
	// summary: Create a hook
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// responses:
	//   "201":
	//     "$ref": "#/responses/Hook"
	if !utils.CheckCreateHookOption(ctx, &form) {
		return
	}
	utils.AddOrgHook(ctx, &form)
}

// EditHook modify a hook of a repository
func EditHook(ctx *context.APIContext, form api.EditHookOption) {
	// swagger:operation PATCH /orgs/{org}/hooks/{id} organization orgEditHook
	// ---
	// summary: Update a hook
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/Hook"
	hookID := ctx.ParamsInt64(":id")
	utils.EditOrgHook(ctx, &form, hookID)
}

// DeleteHook delete a hook of an organization
func DeleteHook(ctx *context.APIContext) {
	// swagger:operation DELETE /orgs/{org}/hooks/{id} organization orgDeleteHook
	// ---
	// summary: Delete a hook
	// produces:
	// - application/json
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	org := ctx.Org.Organization
	hookID := ctx.ParamsInt64(":id")
	if err := models.DeleteWebhookByOrgID(org.ID, hookID); err != nil {
		if models.IsErrWebhookNotExist(err) {
			ctx.Status(404)
		} else {
			ctx.Error(500, "DeleteWebhookByOrgID", err)
		}
		return
	}
	ctx.Status(204)
}
