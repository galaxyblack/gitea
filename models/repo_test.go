// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"path"
	"testing"

	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/setting"

	"github.com/Unknwon/com"
	"github.com/stretchr/testify/assert"
)

func TestRepo(t *testing.T) {
	repo := &Repository{Name: "testRepo"}
	repo.Owner = &User{Name: "testOwner"}

	repo.Units = nil
	assert.Nil(t, repo.ComposeMetas())

	externalTracker := RepoUnit{
		Type: UnitTypeExternalTracker,
		Config: &ExternalTrackerConfig{
			ExternalTrackerFormat: "https://someurl.com/{user}/{repo}/{issue}",
		},
	}

	testSuccess := func(expectedStyle string) {
		repo.Units = []*RepoUnit{&externalTracker}
		repo.ExternalMetas = nil
		metas := repo.ComposeMetas()
		assert.Equal(t, expectedStyle, metas["style"])
		assert.Equal(t, "testRepo", metas["repo"])
		assert.Equal(t, "testOwner", metas["user"])
		assert.Equal(t, "https://someurl.com/{user}/{repo}/{issue}", metas["format"])
	}

	testSuccess(markup.IssueNameStyleNumeric)

	externalTracker.ExternalTrackerConfig().ExternalTrackerStyle = markup.IssueNameStyleAlphanumeric
	testSuccess(markup.IssueNameStyleAlphanumeric)

	externalTracker.ExternalTrackerConfig().ExternalTrackerStyle = markup.IssueNameStyleNumeric
	testSuccess(markup.IssueNameStyleNumeric)
}

func TestGetRepositoryCount(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	count, err1 := GetRepositoryCount(&User{ID: int64(10)})
	privateCount, err2 := GetPrivateRepositoryCount(&User{ID: int64(10)})
	publicCount, err3 := GetPublicRepositoryCount(&User{ID: int64(10)})
	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.NoError(t, err3)
	assert.Equal(t, int64(3), count)
	assert.Equal(t, (privateCount + publicCount), count)
}

func TestGetPublicRepositoryCount(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	count, err := GetPublicRepositoryCount(&User{ID: int64(10)})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestGetPrivateRepositoryCount(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	count, err := GetPrivateRepositoryCount(&User{ID: int64(10)})
	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

func TestUpdateRepositoryVisibilityChanged(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	// Get sample repo and change visibility
	repo, err := GetRepositoryByID(9)
	repo.IsPrivate = true

	// Update it
	err = UpdateRepository(repo, true)
	assert.NoError(t, err)

	// Check visibility of action has become private
	act := Action{}
	_, err = x.ID(3).Get(&act)

	assert.NoError(t, err)
	assert.Equal(t, true, act.IsPrivate)
}

func TestGetUserFork(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	// User13 has repo 11 forked from repo10
	repo, err := GetRepositoryByID(10)
	assert.NoError(t, err)
	assert.NotNil(t, repo)
	repo, err = repo.GetUserFork(13)
	assert.NoError(t, err)
	assert.NotNil(t, repo)

	repo, err = GetRepositoryByID(9)
	assert.NoError(t, err)
	assert.NotNil(t, repo)
	repo, err = repo.GetUserFork(13)
	assert.NoError(t, err)
	assert.Nil(t, repo)
}

func TestForkRepository(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	// user 13 has already forked repo10
	user := AssertExistsAndLoadBean(t, &User{ID: 13}).(*User)
	repo := AssertExistsAndLoadBean(t, &Repository{ID: 10}).(*Repository)

	fork, err := ForkRepository(user, user, repo, "test", "test")
	assert.Nil(t, fork)
	assert.Error(t, err)
	assert.True(t, IsErrRepoAlreadyExist(err))
}

func TestRepoAPIURL(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	repo := AssertExistsAndLoadBean(t, &Repository{ID: 10}).(*Repository)

	assert.Equal(t, "https://try.gitea.io/api/v1/repos/user12/repo10", repo.APIURL())
}

func TestRepoLocalCopyPath(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	repo, err := GetRepositoryByID(10)
	assert.NoError(t, err)
	assert.NotNil(t, repo)

	// test default
	repoID := com.ToStr(repo.ID)
	expected := path.Join(setting.AppDataPath, setting.Repository.Local.LocalCopyPath, repoID)
	assert.Equal(t, expected, repo.LocalCopyPath())

	// test absolute setting
	tempPath := "/tmp/gitea/local-copy-path"
	expected = path.Join(tempPath, repoID)
	setting.Repository.Local.LocalCopyPath = tempPath
	assert.Equal(t, expected, repo.LocalCopyPath())
}

func TestTransferOwnership(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	doer := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	repo := AssertExistsAndLoadBean(t, &Repository{ID: 3}).(*Repository)
	repo.Owner = AssertExistsAndLoadBean(t, &User{ID: repo.OwnerID}).(*User)
	assert.NoError(t, TransferOwnership(doer, "user2", repo))

	transferredRepo := AssertExistsAndLoadBean(t, &Repository{ID: 3}).(*Repository)
	assert.EqualValues(t, 2, transferredRepo.OwnerID)

	assert.False(t, com.IsExist(RepoPath("user3", "repo3")))
	assert.True(t, com.IsExist(RepoPath("user2", "repo3")))
	AssertExistsAndLoadBean(t, &Action{
		OpType:    ActionTransferRepo,
		ActUserID: 2,
		RepoID:    3,
		Content:   "user3/repo3",
	})

	CheckConsistencyFor(t, &Repository{}, &User{}, &Team{})
}
