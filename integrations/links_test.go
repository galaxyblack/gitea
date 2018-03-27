// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
	"path"
	"testing"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"
	api "code.gitea.io/sdk/gitea"

	"github.com/stretchr/testify/assert"
)

func TestLinksNoLogin(t *testing.T) {
	prepareTestEnv(t)

	var links = []string{
		"/explore/repos",
		"/explore/repos?q=test&tab=",
		"/explore/users",
		"/explore/users?q=test&tab=",
		"/explore/organizations",
		"/explore/organizations?q=test&tab=",
		"/",
		"/user/sign_up",
		"/user/login",
		"/user/forgot_password",
		"/api/swagger",
		"/api/v1/swagger",
		// TODO: follow this page and test every link
		"/vendor/librejs.html",
	}

	for _, link := range links {
		req := NewRequest(t, "GET", link)
		MakeRequest(t, req, http.StatusOK)
	}
}

func TestRedirectsNoLogin(t *testing.T) {
	prepareTestEnv(t)

	var redirects = map[string]string{
		"/user2/repo1/commits/master":                "/user2/repo1/commits/branch/master",
		"/user2/repo1/src/master":                    "/user2/repo1/src/branch/master",
		"/user2/repo1/src/master/file.txt":           "/user2/repo1/src/branch/master/file.txt",
		"/user2/repo1/src/master/directory/file.txt": "/user2/repo1/src/branch/master/directory/file.txt",
	}
	for link, redirectLink := range redirects {
		req := NewRequest(t, "GET", link)
		resp := MakeRequest(t, req, http.StatusFound)
		assert.EqualValues(t, path.Join(setting.AppSubURL, redirectLink), test.RedirectURL(resp))
	}
}

func testLinksAsUser(userName string, t *testing.T) {
	var links = []string{
		"/explore/repos",
		"/explore/repos?q=test&tab=",
		"/explore/users",
		"/explore/users?q=test&tab=",
		"/explore/organizations",
		"/explore/organizations?q=test&tab=",
		"/",
		"/user/forgot_password",
		"/api/swagger",
		"/api/v1/swagger",
		"/issues",
		"/issues?type=your_repositories&repo=0&sort=&state=open",
		"/issues?type=assigned&repo=0&sort=&state=open",
		"/issues?type=created_by&repo=0&sort=&state=open",
		"/issues?type=your_repositories&repo=0&sort=&state=closed",
		"/issues?type=assigned&repo=0&sort=&state=closed",
		"/issues?type=created_by&repo=0&sort=&state=closed",
		"/pulls",
		"/pulls?type=your_repositories&repo=0&sort=&state=open",
		"/pulls?type=assigned&repo=0&sort=&state=open",
		"/pulls?type=created_by&repo=0&sort=&state=open",
		"/pulls?type=your_repositories&repo=0&sort=&state=closed",
		"/pulls?type=assigned&repo=0&sort=&state=closed",
		"/pulls?type=created_by&repo=0&sort=&state=closed",
		"/notifications",
		"/repo/create",
		"/repo/migrate",
		"/org/create",
		"/user2",
		"/user2?tab=stars",
		"/user2?tab=activity",
		"/user/settings",
		"/user/settings/avatar",
		"/user/settings/security",
		"/user/settings/security/two_factor/enroll",
		"/user/settings/email",
		"/user/settings/keys",
		"/user/settings/applications",
		"/user/settings/account_link",
		"/user/settings/organization",
		"/user/settings/delete",
	}

	session := loginUser(t, userName)
	for _, link := range links {
		req := NewRequest(t, "GET", link)
		session.MakeRequest(t, req, http.StatusOK)
	}

	reqAPI := NewRequestf(t, "GET", "/api/v1/users/%s/repos", userName)
	respAPI := MakeRequest(t, reqAPI, http.StatusOK)

	var apiRepos []api.Repository
	DecodeJSON(t, respAPI, &apiRepos)

	var repoLinks = []string{
		"",
		"/issues",
		"/pulls",
		"/commits/branch/master",
		"/graph",
		"/settings",
		"/settings/collaboration",
		"/settings/branches",
		"/settings/hooks",
		// FIXME: below links should return 200 but 404 ??
		//"/settings/hooks/git",
		//"/settings/hooks/git/pre-receive",
		//"/settings/hooks/git/update",
		//"/settings/hooks/git/post-receive",
		"/settings/keys",
		"/releases",
		"/releases/new",
		//"/wiki/_pages",
		"/wiki/_new",
	}

	for _, repo := range apiRepos {
		for _, link := range repoLinks {
			req := NewRequest(t, "GET", fmt.Sprintf("/%s/%s%s", userName, repo.Name, link))
			session.MakeRequest(t, req, http.StatusOK)
		}
	}
}

func TestLinksLogin(t *testing.T) {
	prepareTestEnv(t)

	testLinksAsUser("user2", t)
}
