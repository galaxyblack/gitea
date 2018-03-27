// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"math/rand"
	"testing"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
)

func TestGetUserEmailsByNames(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	// ignore none active user email
	assert.Equal(t, []string{"user8@example.com"}, GetUserEmailsByNames([]string{"user8", "user9"}))
	assert.Equal(t, []string{"user8@example.com", "user5@example.com"}, GetUserEmailsByNames([]string{"user8", "user5"}))
}

func TestCanCreateOrganization(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	admin := AssertExistsAndLoadBean(t, &User{ID: 1}).(*User)
	assert.True(t, admin.CanCreateOrganization())

	user := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	assert.True(t, user.CanCreateOrganization())
	// Disable user create organization permission.
	user.AllowCreateOrganization = false
	assert.False(t, user.CanCreateOrganization())

	setting.Admin.DisableRegularOrgCreation = true
	user.AllowCreateOrganization = true
	assert.True(t, admin.CanCreateOrganization())
	assert.False(t, user.CanCreateOrganization())
}

func TestSearchUsers(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	testSuccess := func(opts *SearchUserOptions, expectedUserOrOrgIDs []int64) {
		users, _, err := SearchUsers(opts)
		assert.NoError(t, err)
		if assert.Len(t, users, len(expectedUserOrOrgIDs)) {
			for i, expectedID := range expectedUserOrOrgIDs {
				assert.EqualValues(t, expectedID, users[i].ID)
			}
		}
	}

	// test orgs
	testOrgSuccess := func(opts *SearchUserOptions, expectedOrgIDs []int64) {
		opts.Type = UserTypeOrganization
		testSuccess(opts, expectedOrgIDs)
	}

	testOrgSuccess(&SearchUserOptions{OrderBy: "id ASC", Page: 1, PageSize: 2},
		[]int64{3, 6})

	testOrgSuccess(&SearchUserOptions{OrderBy: "id ASC", Page: 2, PageSize: 2},
		[]int64{7, 17})

	testOrgSuccess(&SearchUserOptions{OrderBy: "id ASC", Page: 3, PageSize: 2},
		[]int64{19})

	testOrgSuccess(&SearchUserOptions{Page: 4, PageSize: 2},
		[]int64{})

	// test users
	testUserSuccess := func(opts *SearchUserOptions, expectedUserIDs []int64) {
		opts.Type = UserTypeIndividual
		testSuccess(opts, expectedUserIDs)
	}

	testUserSuccess(&SearchUserOptions{OrderBy: "id ASC", Page: 1},
		[]int64{1, 2, 4, 5, 8, 9, 10, 11, 12, 13, 14, 15, 16, 18, 20})

	testUserSuccess(&SearchUserOptions{Page: 1, IsActive: util.OptionalBoolFalse},
		[]int64{9})

	testUserSuccess(&SearchUserOptions{OrderBy: "id ASC", Page: 1, IsActive: util.OptionalBoolTrue},
		[]int64{1, 2, 4, 5, 8, 10, 11, 12, 13, 14, 15, 16, 18, 20})

	testUserSuccess(&SearchUserOptions{Keyword: "user1", OrderBy: "id ASC", Page: 1, IsActive: util.OptionalBoolTrue},
		[]int64{1, 10, 11, 12, 13, 14, 15, 16, 18})

	// order by name asc default
	testUserSuccess(&SearchUserOptions{Keyword: "user1", Page: 1, IsActive: util.OptionalBoolTrue},
		[]int64{1, 10, 11, 12, 13, 14, 15, 16, 18})
}

func TestDeleteUser(t *testing.T) {
	test := func(userID int64) {
		assert.NoError(t, PrepareTestDatabase())
		user := AssertExistsAndLoadBean(t, &User{ID: userID}).(*User)

		ownedRepos := make([]*Repository, 0, 10)
		assert.NoError(t, x.Find(&ownedRepos, &Repository{OwnerID: userID}))
		if len(ownedRepos) > 0 {
			err := DeleteUser(user)
			assert.Error(t, err)
			assert.True(t, IsErrUserOwnRepos(err))
			return
		}

		orgUsers := make([]*OrgUser, 0, 10)
		assert.NoError(t, x.Find(&orgUsers, &OrgUser{UID: userID}))
		for _, orgUser := range orgUsers {
			if err := RemoveOrgUser(orgUser.OrgID, orgUser.UID); err != nil {
				assert.True(t, IsErrLastOrgOwner(err))
				return
			}
		}
		assert.NoError(t, DeleteUser(user))
		AssertNotExistsBean(t, &User{ID: userID})
		CheckConsistencyFor(t, &User{}, &Repository{})
	}
	test(2)
	test(4)
	test(8)
	test(11)
}

func TestHashPasswordDeterministic(t *testing.T) {
	b := make([]byte, 16)
	rand.Read(b)
	u := &User{Salt: string(b)}
	for i := 0; i < 50; i++ {
		// generate a random password
		rand.Read(b)
		pass := string(b)

		// save the current password in the user - hash it and store the result
		u.HashPassword(pass)
		r1 := u.Passwd

		// run again
		u.HashPassword(pass)
		r2 := u.Passwd

		// assert equal (given the same salt+pass, the same result is produced)
		assert.Equal(t, r1, r2)
	}
}

func BenchmarkHashPassword(b *testing.B) {
	// BenchmarkHashPassword ensures that it takes a reasonable amount of time
	// to hash a password - in order to protect from brute-force attacks.
	pass := "password1337"
	bs := make([]byte, 16)
	rand.Read(bs)
	u := &User{Salt: string(bs), Passwd: pass}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		u.HashPassword(pass)
	}
}
