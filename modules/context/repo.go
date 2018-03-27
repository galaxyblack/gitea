// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package context

import (
	"fmt"
	"io/ioutil"
	"path"
	"strings"

	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/Unknwon/com"
	"gopkg.in/editorconfig/editorconfig-core-go.v1"
	"gopkg.in/macaron.v1"
)

// PullRequest contains informations to make a pull request
type PullRequest struct {
	BaseRepo *models.Repository
	Allowed  bool
	SameRepo bool
	HeadInfo string // [<user>:]<branch>
}

// Repository contains information to operate a repository
type Repository struct {
	AccessMode   models.AccessMode
	IsWatching   bool
	IsViewBranch bool
	IsViewTag    bool
	IsViewCommit bool
	Repository   *models.Repository
	Owner        *models.User
	Commit       *git.Commit
	Tag          *git.Tag
	GitRepo      *git.Repository
	BranchName   string
	TagName      string
	TreePath     string
	CommitID     string
	RepoLink     string
	CloneLink    models.CloneLink
	CommitsCount int64
	Mirror       *models.Mirror

	PullRequest *PullRequest
}

// IsOwner returns true if current user is the owner of repository.
func (r *Repository) IsOwner() bool {
	return r.AccessMode >= models.AccessModeOwner
}

// IsAdmin returns true if current user has admin or higher access of repository.
func (r *Repository) IsAdmin() bool {
	return r.AccessMode >= models.AccessModeAdmin
}

// IsWriter returns true if current user has write or higher access of repository.
func (r *Repository) IsWriter() bool {
	return r.AccessMode >= models.AccessModeWrite
}

// HasAccess returns true if the current user has at least read access for this repository
func (r *Repository) HasAccess() bool {
	return r.AccessMode >= models.AccessModeRead
}

// CanEnableEditor returns true if repository is editable and user has proper access level.
func (r *Repository) CanEnableEditor() bool {
	return r.Repository.CanEnableEditor() && r.IsViewBranch && r.IsWriter()
}

// CanCreateBranch returns true if repository is editable and user has proper access level.
func (r *Repository) CanCreateBranch() bool {
	return r.Repository.CanCreateBranch() && r.IsWriter()
}

// CanCommitToBranch returns true if repository is editable and user has proper access level
//   and branch is not protected
func (r *Repository) CanCommitToBranch(doer *models.User) (bool, error) {
	protectedBranch, err := r.Repository.IsProtectedBranch(r.BranchName, doer)
	if err != nil {
		return false, err
	}
	return r.CanEnableEditor() && !protectedBranch, nil
}

// CanUseTimetracker returns whether or not a user can use the timetracker.
func (r *Repository) CanUseTimetracker(issue *models.Issue, user *models.User) bool {
	// Checking for following:
	// 1. Is timetracker enabled
	// 2. Is the user a contributor, admin, poster or assignee and do the repository policies require this?
	return r.Repository.IsTimetrackerEnabled() && (!r.Repository.AllowOnlyContributorsToTrackTime() ||
		r.IsWriter() || issue.IsPoster(user.ID) || issue.AssigneeID == user.ID)
}

// GetCommitsCount returns cached commit count for current view
func (r *Repository) GetCommitsCount() (int64, error) {
	var contextName string
	if r.IsViewBranch {
		contextName = r.BranchName
	} else if r.IsViewTag {
		contextName = r.TagName
	} else {
		contextName = r.CommitID
	}
	return cache.GetInt64(r.Repository.GetCommitsCountCacheKey(contextName, r.IsViewBranch || r.IsViewTag), func() (int64, error) {
		return r.Commit.CommitsCount()
	})
}

// BranchNameSubURL sub-URL for the BranchName field
func (r *Repository) BranchNameSubURL() string {
	switch {
	case r.IsViewBranch:
		return "branch/" + r.BranchName
	case r.IsViewTag:
		return "tag/" + r.BranchName
	case r.IsViewCommit:
		return "commit/" + r.BranchName
	}
	log.Error(4, "Unknown view type for repo: %v", r)
	return ""
}

// GetEditorconfig returns the .editorconfig definition if found in the
// HEAD of the default repo branch.
func (r *Repository) GetEditorconfig() (*editorconfig.Editorconfig, error) {
	commit, err := r.GitRepo.GetBranchCommit(r.Repository.DefaultBranch)
	if err != nil {
		return nil, err
	}
	treeEntry, err := commit.GetTreeEntryByPath(".editorconfig")
	if err != nil {
		return nil, err
	}
	if treeEntry.Blob().Size() >= setting.UI.MaxDisplayFileSize {
		return nil, git.ErrNotExist{ID: "", RelPath: ".editorconfig"}
	}
	reader, err := treeEntry.Blob().Data()
	if err != nil {
		return nil, err
	}
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return editorconfig.ParseBytes(data)
}

// RetrieveBaseRepo retrieves base repository
func RetrieveBaseRepo(ctx *Context, repo *models.Repository) {
	// Non-fork repository will not return error in this method.
	if err := repo.GetBaseRepo(); err != nil {
		if models.IsErrRepoNotExist(err) {
			repo.IsFork = false
			repo.ForkID = 0
			return
		}
		ctx.ServerError("GetBaseRepo", err)
		return
	} else if err = repo.BaseRepo.GetOwner(); err != nil {
		ctx.ServerError("BaseRepo.GetOwner", err)
		return
	}
}

// ComposeGoGetImport returns go-get-import meta content.
func ComposeGoGetImport(owner, repo string) string {
	return path.Join(setting.Domain, setting.AppSubURL, owner, repo)
}

// EarlyResponseForGoGetMeta responses appropriate go-get meta with status 200
// if user does not have actual access to the requested repository,
// or the owner or repository does not exist at all.
// This is particular a workaround for "go get" command which does not respect
// .netrc file.
func EarlyResponseForGoGetMeta(ctx *Context) {
	username := ctx.Params(":username")
	reponame := ctx.Params(":reponame")
	ctx.PlainText(200, []byte(com.Expand(`<meta name="go-import" content="{GoGetImport} git {CloneLink}">`,
		map[string]string{
			"GoGetImport": ComposeGoGetImport(username, strings.TrimSuffix(reponame, ".git")),
			"CloneLink":   models.ComposeHTTPSCloneURL(username, reponame),
		})))
}

// RedirectToRepo redirect to a differently-named repository
func RedirectToRepo(ctx *Context, redirectRepoID int64) {
	ownerName := ctx.Params(":username")
	previousRepoName := ctx.Params(":reponame")

	repo, err := models.GetRepositoryByID(redirectRepoID)
	if err != nil {
		ctx.ServerError("GetRepositoryByID", err)
		return
	}

	redirectPath := strings.Replace(
		ctx.Req.URL.Path,
		fmt.Sprintf("%s/%s", ownerName, previousRepoName),
		fmt.Sprintf("%s/%s", ownerName, repo.Name),
		1,
	)
	ctx.Redirect(redirectPath)
}

func repoAssignment(ctx *Context, repo *models.Repository) {
	// Admin has super access.
	if ctx.IsSigned && ctx.User.IsAdmin {
		ctx.Repo.AccessMode = models.AccessModeOwner
	} else {
		var userID int64
		if ctx.User != nil {
			userID = ctx.User.ID
		}
		mode, err := models.AccessLevel(userID, repo)
		if err != nil {
			ctx.ServerError("AccessLevel", err)
			return
		}
		ctx.Repo.AccessMode = mode
	}

	// Check access.
	if ctx.Repo.AccessMode == models.AccessModeNone {
		if ctx.Query("go-get") == "1" {
			EarlyResponseForGoGetMeta(ctx)
			return
		}
		ctx.NotFound("no access right", nil)
		return
	}
	ctx.Data["HasAccess"] = true

	if repo.IsMirror {
		var err error
		ctx.Repo.Mirror, err = models.GetMirrorByRepoID(repo.ID)
		if err != nil {
			ctx.ServerError("GetMirror", err)
			return
		}
		ctx.Data["MirrorEnablePrune"] = ctx.Repo.Mirror.EnablePrune
		ctx.Data["MirrorInterval"] = ctx.Repo.Mirror.Interval
		ctx.Data["Mirror"] = ctx.Repo.Mirror
	}

	ctx.Repo.Repository = repo
	ctx.Data["RepoName"] = ctx.Repo.Repository.Name
	ctx.Data["IsBareRepo"] = ctx.Repo.Repository.IsBare
}

// RepoIDAssignment returns a macaron handler which assigns the repo to the context.
func RepoIDAssignment() macaron.Handler {
	return func(ctx *Context) {
		repoID := ctx.ParamsInt64(":repoid")

		// Get repository.
		repo, err := models.GetRepositoryByID(repoID)
		if err != nil {
			if models.IsErrRepoNotExist(err) {
				ctx.NotFound("GetRepositoryByID", nil)
			} else {
				ctx.ServerError("GetRepositoryByID", err)
			}
			return
		}

		if err = repo.GetOwner(); err != nil {
			ctx.ServerError("GetOwner", err)
			return
		}
		repoAssignment(ctx, repo)
	}
}

// RepoAssignment returns a macaron to handle repository assignment
func RepoAssignment() macaron.Handler {
	return func(ctx *Context) {
		var (
			owner *models.User
			err   error
		)

		userName := ctx.Params(":username")
		repoName := ctx.Params(":reponame")

		// Check if the user is the same as the repository owner
		if ctx.IsSigned && ctx.User.LowerName == strings.ToLower(userName) {
			owner = ctx.User
		} else {
			owner, err = models.GetUserByName(userName)
			if err != nil {
				if models.IsErrUserNotExist(err) {
					if ctx.Query("go-get") == "1" {
						EarlyResponseForGoGetMeta(ctx)
						return
					}
					ctx.NotFound("GetUserByName", nil)
				} else {
					ctx.ServerError("GetUserByName", err)
				}
				return
			}
		}
		ctx.Repo.Owner = owner
		ctx.Data["Username"] = ctx.Repo.Owner.Name

		// Get repository.
		repo, err := models.GetRepositoryByName(owner.ID, repoName)
		if err != nil {
			if models.IsErrRepoNotExist(err) {
				redirectRepoID, err := models.LookupRepoRedirect(owner.ID, repoName)
				if err == nil {
					RedirectToRepo(ctx, redirectRepoID)
				} else if models.IsErrRepoRedirectNotExist(err) {
					if ctx.Query("go-get") == "1" {
						EarlyResponseForGoGetMeta(ctx)
						return
					}
					ctx.NotFound("GetRepositoryByName", nil)
				} else {
					ctx.ServerError("LookupRepoRedirect", err)
				}
			} else {
				ctx.ServerError("GetRepositoryByName", err)
			}
			return
		}
		repo.Owner = owner

		repoAssignment(ctx, repo)
		if ctx.Written() {
			return
		}

		gitRepo, err := git.OpenRepository(models.RepoPath(userName, repoName))
		if err != nil {
			ctx.ServerError("RepoAssignment Invalid repo "+models.RepoPath(userName, repoName), err)
			return
		}
		ctx.Repo.GitRepo = gitRepo
		ctx.Repo.RepoLink = repo.Link()
		ctx.Data["RepoLink"] = ctx.Repo.RepoLink
		ctx.Data["RepoRelPath"] = ctx.Repo.Owner.Name + "/" + ctx.Repo.Repository.Name

		tags, err := ctx.Repo.GitRepo.GetTags()
		if err != nil {
			ctx.ServerError("GetTags", err)
			return
		}
		ctx.Data["Tags"] = tags

		count, err := models.GetReleaseCountByRepoID(ctx.Repo.Repository.ID, models.FindReleasesOptions{
			IncludeDrafts: false,
			IncludeTags:   true,
		})
		if err != nil {
			ctx.ServerError("GetReleaseCountByRepoID", err)
			return
		}
		ctx.Repo.Repository.NumReleases = int(count)

		ctx.Data["Title"] = owner.Name + "/" + repo.Name
		ctx.Data["Repository"] = repo
		ctx.Data["Owner"] = ctx.Repo.Repository.Owner
		ctx.Data["IsRepositoryOwner"] = ctx.Repo.IsOwner()
		ctx.Data["IsRepositoryAdmin"] = ctx.Repo.IsAdmin()
		ctx.Data["IsRepositoryWriter"] = ctx.Repo.IsWriter()

		if ctx.Data["CanSignedUserFork"], err = ctx.Repo.Repository.CanUserFork(ctx.User); err != nil {
			ctx.ServerError("CanUserFork", err)
			return
		}

		ctx.Data["DisableSSH"] = setting.SSH.Disabled
		ctx.Data["ExposeAnonSSH"] = setting.SSH.ExposeAnonymous
		ctx.Data["DisableHTTP"] = setting.Repository.DisableHTTPGit
		ctx.Data["RepoSearchEnabled"] = setting.Indexer.RepoIndexerEnabled
		ctx.Data["CloneLink"] = repo.CloneLink()
		ctx.Data["WikiCloneLink"] = repo.WikiCloneLink()

		if ctx.IsSigned {
			ctx.Data["IsWatchingRepo"] = models.IsWatching(ctx.User.ID, repo.ID)
			ctx.Data["IsStaringRepo"] = models.IsStaring(ctx.User.ID, repo.ID)
		}

		// repo is bare and display enable
		if ctx.Repo.Repository.IsBare {
			ctx.Data["BranchName"] = ctx.Repo.Repository.DefaultBranch
			return
		}

		ctx.Data["TagName"] = ctx.Repo.TagName
		brs, err := ctx.Repo.GitRepo.GetBranches()
		if err != nil {
			ctx.ServerError("GetBranches", err)
			return
		}
		ctx.Data["Branches"] = brs
		ctx.Data["BranchesCount"] = len(brs)

		// If not branch selected, try default one.
		// If default branch doesn't exists, fall back to some other branch.
		if len(ctx.Repo.BranchName) == 0 {
			if len(ctx.Repo.Repository.DefaultBranch) > 0 && gitRepo.IsBranchExist(ctx.Repo.Repository.DefaultBranch) {
				ctx.Repo.BranchName = ctx.Repo.Repository.DefaultBranch
			} else if len(brs) > 0 {
				ctx.Repo.BranchName = brs[0]
			}
		}
		ctx.Data["BranchName"] = ctx.Repo.BranchName
		ctx.Data["CommitID"] = ctx.Repo.CommitID

		if repo.IsFork {
			RetrieveBaseRepo(ctx, repo)
			if ctx.Written() {
				return
			}
		}

		// People who have push access or have forked repository can propose a new pull request.
		if ctx.Repo.IsWriter() || (ctx.IsSigned && ctx.User.HasForkedRepo(ctx.Repo.Repository.ID)) {
			// Pull request is allowed if this is a fork repository
			// and base repository accepts pull requests.
			if repo.BaseRepo != nil && repo.BaseRepo.AllowsPulls() {
				ctx.Data["BaseRepo"] = repo.BaseRepo
				ctx.Repo.PullRequest.BaseRepo = repo.BaseRepo
				ctx.Repo.PullRequest.Allowed = true
				ctx.Repo.PullRequest.HeadInfo = ctx.Repo.Owner.Name + ":" + ctx.Repo.BranchName
			} else {
				// Or, this is repository accepts pull requests between branches.
				if repo.AllowsPulls() {
					ctx.Data["BaseRepo"] = repo
					ctx.Repo.PullRequest.BaseRepo = repo
					ctx.Repo.PullRequest.Allowed = true
					ctx.Repo.PullRequest.SameRepo = true
					ctx.Repo.PullRequest.HeadInfo = ctx.Repo.BranchName
				}
			}

			// Reset repo units as otherwise user specific units wont be loaded later
			ctx.Repo.Repository.Units = nil
		}
		ctx.Data["PullRequestCtx"] = ctx.Repo.PullRequest

		if ctx.Query("go-get") == "1" {
			ctx.Data["GoGetImport"] = ComposeGoGetImport(owner.Name, repo.Name)
			prefix := setting.AppURL + path.Join(owner.Name, repo.Name, "src", "branch", ctx.Repo.BranchName)
			ctx.Data["GoDocDirectory"] = prefix + "{/dir}"
			ctx.Data["GoDocFile"] = prefix + "{/dir}/{file}#L{line}"
		}
	}
}

// RepoRefType type of repo reference
type RepoRefType int

const (
	// RepoRefLegacy unknown type, make educated guess and redirect.
	// for backward compatibility with previous URL scheme
	RepoRefLegacy RepoRefType = iota
	// RepoRefAny is for usage where educated guess is needed
	// but redirect can not be made
	RepoRefAny
	// RepoRefBranch branch
	RepoRefBranch
	// RepoRefTag tag
	RepoRefTag
	// RepoRefCommit commit
	RepoRefCommit
)

// RepoRef handles repository reference names when the ref name is not
// explicitly given
func RepoRef() macaron.Handler {
	// since no ref name is explicitly specified, ok to just use branch
	return RepoRefByType(RepoRefBranch)
}

func getRefNameFromPath(ctx *Context, path string, isExist func(string) bool) string {
	refName := ""
	parts := strings.Split(path, "/")
	for i, part := range parts {
		refName = strings.TrimPrefix(refName+"/"+part, "/")
		if isExist(refName) {
			ctx.Repo.TreePath = strings.Join(parts[i+1:], "/")
			return refName
		}
	}
	return ""
}

func getRefName(ctx *Context, pathType RepoRefType) string {
	path := ctx.Params("*")
	switch pathType {
	case RepoRefLegacy, RepoRefAny:
		if refName := getRefName(ctx, RepoRefBranch); len(refName) > 0 {
			return refName
		}
		if refName := getRefName(ctx, RepoRefTag); len(refName) > 0 {
			return refName
		}
		if refName := getRefName(ctx, RepoRefCommit); len(refName) > 0 {
			return refName
		}
		ctx.Repo.TreePath = path
		return ctx.Repo.Repository.DefaultBranch
	case RepoRefBranch:
		return getRefNameFromPath(ctx, path, ctx.Repo.GitRepo.IsBranchExist)
	case RepoRefTag:
		return getRefNameFromPath(ctx, path, ctx.Repo.GitRepo.IsTagExist)
	case RepoRefCommit:
		parts := strings.Split(path, "/")
		if len(parts) > 0 && len(parts[0]) == 40 {
			ctx.Repo.TreePath = strings.Join(parts[1:], "/")
			return parts[0]
		}
	default:
		log.Error(4, "Unrecognized path type: %v", path)
	}
	return ""
}

// RepoRefByType handles repository reference name for a specific type
// of repository reference
func RepoRefByType(refType RepoRefType) macaron.Handler {
	return func(ctx *Context) {
		// Empty repository does not have reference information.
		if ctx.Repo.Repository.IsBare {
			return
		}

		var (
			refName string
			err     error
		)

		// For API calls.
		if ctx.Repo.GitRepo == nil {
			repoPath := models.RepoPath(ctx.Repo.Owner.Name, ctx.Repo.Repository.Name)
			ctx.Repo.GitRepo, err = git.OpenRepository(repoPath)
			if err != nil {
				ctx.ServerError("RepoRef Invalid repo "+repoPath, err)
				return
			}
		}

		// Get default branch.
		if len(ctx.Params("*")) == 0 {
			refName = ctx.Repo.Repository.DefaultBranch
			ctx.Repo.BranchName = refName
			if !ctx.Repo.GitRepo.IsBranchExist(refName) {
				brs, err := ctx.Repo.GitRepo.GetBranches()
				if err != nil {
					ctx.ServerError("GetBranches", err)
					return
				} else if len(brs) == 0 {
					err = fmt.Errorf("No branches in non-bare repository %s",
						ctx.Repo.GitRepo.Path)
					ctx.ServerError("GetBranches", err)
					return
				}
				refName = brs[0]
			}
			ctx.Repo.Commit, err = ctx.Repo.GitRepo.GetBranchCommit(refName)
			if err != nil {
				ctx.ServerError("GetBranchCommit", err)
				return
			}
			ctx.Repo.CommitID = ctx.Repo.Commit.ID.String()
			ctx.Repo.IsViewBranch = true

		} else {
			refName = getRefName(ctx, refType)
			ctx.Repo.BranchName = refName
			if ctx.Repo.GitRepo.IsBranchExist(refName) {
				ctx.Repo.IsViewBranch = true

				ctx.Repo.Commit, err = ctx.Repo.GitRepo.GetBranchCommit(refName)
				if err != nil {
					ctx.ServerError("GetBranchCommit", err)
					return
				}
				ctx.Repo.CommitID = ctx.Repo.Commit.ID.String()

			} else if ctx.Repo.GitRepo.IsTagExist(refName) {
				ctx.Repo.IsViewTag = true
				ctx.Repo.Commit, err = ctx.Repo.GitRepo.GetTagCommit(refName)
				if err != nil {
					ctx.ServerError("GetTagCommit", err)
					return
				}
				ctx.Repo.CommitID = ctx.Repo.Commit.ID.String()
			} else if len(refName) == 40 {
				ctx.Repo.IsViewCommit = true
				ctx.Repo.CommitID = refName

				ctx.Repo.Commit, err = ctx.Repo.GitRepo.GetCommit(refName)
				if err != nil {
					ctx.NotFound("GetCommit", nil)
					return
				}
			} else {
				ctx.NotFound("RepoRef invalid repo", fmt.Errorf("branch or tag not exist: %s", refName))
				return
			}

			if refType == RepoRefLegacy {
				// redirect from old URL scheme to new URL scheme
				ctx.Redirect(path.Join(
					setting.AppSubURL,
					strings.TrimSuffix(ctx.Req.URL.String(), ctx.Params("*")),
					ctx.Repo.BranchNameSubURL(),
					ctx.Repo.TreePath))
				return
			}
		}

		ctx.Data["BranchName"] = ctx.Repo.BranchName
		ctx.Data["BranchNameSubURL"] = ctx.Repo.BranchNameSubURL()
		ctx.Data["CommitID"] = ctx.Repo.CommitID
		ctx.Data["TreePath"] = ctx.Repo.TreePath
		ctx.Data["IsViewBranch"] = ctx.Repo.IsViewBranch
		ctx.Data["IsViewTag"] = ctx.Repo.IsViewTag
		ctx.Data["IsViewCommit"] = ctx.Repo.IsViewCommit
		ctx.Data["CanCreateBranch"] = ctx.Repo.CanCreateBranch()

		ctx.Repo.CommitsCount, err = ctx.Repo.GetCommitsCount()
		if err != nil {
			ctx.ServerError("GetCommitsCount", err)
			return
		}
		ctx.Data["CommitsCount"] = ctx.Repo.CommitsCount
	}
}

// RequireRepoAdmin returns a macaron middleware for requiring repository admin permission
func RequireRepoAdmin() macaron.Handler {
	return func(ctx *Context) {
		if !ctx.IsSigned || (!ctx.Repo.IsAdmin() && !ctx.User.IsAdmin) {
			ctx.NotFound(ctx.Req.RequestURI, nil)
			return
		}
	}
}

// RequireRepoWriter returns a macaron middleware for requiring repository write permission
func RequireRepoWriter() macaron.Handler {
	return func(ctx *Context) {
		if !ctx.IsSigned || (!ctx.Repo.IsWriter() && !ctx.User.IsAdmin) {
			ctx.NotFound(ctx.Req.RequestURI, nil)
			return
		}
	}
}

// LoadRepoUnits loads repsitory's units, it should be called after repository and user loaded
func LoadRepoUnits() macaron.Handler {
	return func(ctx *Context) {
		var isAdmin bool
		if ctx.User != nil && ctx.User.IsAdmin {
			isAdmin = true
		}

		var userID int64
		if ctx.User != nil {
			userID = ctx.User.ID
		}
		err := ctx.Repo.Repository.LoadUnitsByUserID(userID, isAdmin)
		if err != nil {
			ctx.ServerError("LoadUnitsByUserID", err)
			return
		}
	}
}

// CheckUnit will check whether unit type is enabled
func CheckUnit(unitType models.UnitType) macaron.Handler {
	return func(ctx *Context) {
		if !ctx.Repo.Repository.UnitEnabled(unitType) {
			ctx.NotFound("CheckUnit", fmt.Errorf("%s: %v", ctx.Tr("units.error.unit_not_allowed"), unitType))
		}
	}
}

// CheckAnyUnit will check whether any of the unit types are enabled
func CheckAnyUnit(unitTypes ...models.UnitType) macaron.Handler {
	return func(ctx *Context) {
		if !ctx.Repo.Repository.AnyUnitEnabled(unitTypes...) {
			ctx.NotFound("CheckAnyUnit", fmt.Errorf("%s: %v", ctx.Tr("units.error.unit_not_allowed"), unitTypes))
		}
	}
}

// GitHookService checks if repository Git hooks service has been enabled.
func GitHookService() macaron.Handler {
	return func(ctx *Context) {
		if !ctx.User.CanEditGitHook() {
			ctx.NotFound("GitHookService", nil)
			return
		}
	}
}

// UnitTypes returns a macaron middleware to set unit types to context variables.
func UnitTypes() macaron.Handler {
	return func(ctx *Context) {
		ctx.Data["UnitTypeCode"] = models.UnitTypeCode
		ctx.Data["UnitTypeIssues"] = models.UnitTypeIssues
		ctx.Data["UnitTypePullRequests"] = models.UnitTypePullRequests
		ctx.Data["UnitTypeReleases"] = models.UnitTypeReleases
		ctx.Data["UnitTypeWiki"] = models.UnitTypeWiki
		ctx.Data["UnitTypeExternalWiki"] = models.UnitTypeExternalWiki
		ctx.Data["UnitTypeExternalTracker"] = models.UnitTypeExternalTracker
	}
}
