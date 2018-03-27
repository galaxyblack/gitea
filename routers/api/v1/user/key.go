// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	api "code.gitea.io/sdk/gitea"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/routers/api/v1/convert"
	"code.gitea.io/gitea/routers/api/v1/repo"
)

// GetUserByParamsName get user by name
func GetUserByParamsName(ctx *context.APIContext, name string) *models.User {
	user, err := models.GetUserByName(ctx.Params(name))
	if err != nil {
		if models.IsErrUserNotExist(err) {
			ctx.Status(404)
		} else {
			ctx.Error(500, "GetUserByName", err)
		}
		return nil
	}
	return user
}

// GetUserByParams returns user whose name is presented in URL paramenter.
func GetUserByParams(ctx *context.APIContext) *models.User {
	return GetUserByParamsName(ctx, ":username")
}

func composePublicKeysAPILink() string {
	return setting.AppURL + "api/v1/user/keys/"
}

func listPublicKeys(ctx *context.APIContext, uid int64) {
	keys, err := models.ListPublicKeys(uid)
	if err != nil {
		ctx.Error(500, "ListPublicKeys", err)
		return
	}

	apiLink := composePublicKeysAPILink()
	apiKeys := make([]*api.PublicKey, len(keys))
	for i := range keys {
		apiKeys[i] = convert.ToPublicKey(apiLink, keys[i])
	}

	ctx.JSON(200, &apiKeys)
}

// ListMyPublicKeys list all of the authenticated user's public keys
func ListMyPublicKeys(ctx *context.APIContext) {
	// swagger:operation GET /user/keys user userCurrentListKeys
	// ---
	// summary: List the authenticated user's public keys
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/PublicKeyList"
	listPublicKeys(ctx, ctx.User.ID)
}

// ListPublicKeys list the given user's public keys
func ListPublicKeys(ctx *context.APIContext) {
	// swagger:operation GET /users/{username}/keys user userListKeys
	// ---
	// summary: List the given user's public keys
	// produces:
	// - application/json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of user
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/PublicKeyList"
	user := GetUserByParams(ctx)
	if ctx.Written() {
		return
	}
	listPublicKeys(ctx, user.ID)
}

// GetPublicKey get a public key
func GetPublicKey(ctx *context.APIContext) {
	// swagger:operation GET /user/keys/{id} user userCurrentGetKey
	// ---
	// summary: Get a public key
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of key to get
	//   type: integer
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/PublicKey"
	//   "404":
	//     "$ref": "#/responses/notFound"
	key, err := models.GetPublicKeyByID(ctx.ParamsInt64(":id"))
	if err != nil {
		if models.IsErrKeyNotExist(err) {
			ctx.Status(404)
		} else {
			ctx.Error(500, "GetPublicKeyByID", err)
		}
		return
	}

	apiLink := composePublicKeysAPILink()
	ctx.JSON(200, convert.ToPublicKey(apiLink, key))
}

// CreateUserPublicKey creates new public key to given user by ID.
func CreateUserPublicKey(ctx *context.APIContext, form api.CreateKeyOption, uid int64) {
	content, err := models.CheckPublicKeyString(form.Key)
	if err != nil {
		repo.HandleCheckKeyStringError(ctx, err)
		return
	}

	key, err := models.AddPublicKey(uid, form.Title, content)
	if err != nil {
		repo.HandleAddKeyError(ctx, err)
		return
	}
	apiLink := composePublicKeysAPILink()
	ctx.JSON(201, convert.ToPublicKey(apiLink, key))
}

// CreatePublicKey create one public key for me
func CreatePublicKey(ctx *context.APIContext, form api.CreateKeyOption) {
	// swagger:operation POST /user/keys user userCurrentPostKey
	// ---
	// summary: Create a public key
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateKeyOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/PublicKey"
	//   "422":
	//     "$ref": "#/responses/validationError"
	CreateUserPublicKey(ctx, form, ctx.User.ID)
}

// DeletePublicKey delete one public key
func DeletePublicKey(ctx *context.APIContext) {
	// swagger:operation DELETE /user/keys/{id} user userCurrentDeleteKey
	// ---
	// summary: Delete a public key
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of key to delete
	//   type: integer
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	if err := models.DeletePublicKey(ctx.User, ctx.ParamsInt64(":id")); err != nil {
		if models.IsErrKeyNotExist(err) {
			ctx.Status(404)
		} else if models.IsErrKeyAccessDenied(err) {
			ctx.Error(403, "", "You do not have access to this key")
		} else {
			ctx.Error(500, "DeletePublicKey", err)
		}
		return
	}

	ctx.Status(204)
}
