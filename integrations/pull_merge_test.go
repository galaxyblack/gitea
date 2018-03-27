// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
)

func testPullMerge(t *testing.T, session *TestSession, user, repo, pullnum string, mergeStyle models.MergeStyle) *httptest.ResponseRecorder {
	req := NewRequest(t, "GET", path.Join(user, repo, "pulls", pullnum))
	resp := session.MakeRequest(t, req, http.StatusOK)

	// Click the little green button to create a pull
	htmlDoc := NewHTMLParser(t, resp.Body)
	link, exists := htmlDoc.doc.Find(".ui.form." + string(mergeStyle) + "-fields > form").Attr("action")
	assert.True(t, exists, "The template has changed")
	req = NewRequestWithValues(t, "POST", link, map[string]string{
		"_csrf": htmlDoc.GetCSRF(),
		"do":    string(mergeStyle),
	})
	resp = session.MakeRequest(t, req, http.StatusFound)

	return resp
}

func testPullCleanUp(t *testing.T, session *TestSession, user, repo, pullnum string) *httptest.ResponseRecorder {
	req := NewRequest(t, "GET", path.Join(user, repo, "pulls", pullnum))
	resp := session.MakeRequest(t, req, http.StatusOK)

	// Click the little green button to create a pull
	htmlDoc := NewHTMLParser(t, resp.Body)
	link, exists := htmlDoc.doc.Find(".comments .merge .delete-button").Attr("data-url")
	assert.True(t, exists, "The template has changed")
	req = NewRequestWithValues(t, "POST", link, map[string]string{
		"_csrf": htmlDoc.GetCSRF(),
	})
	resp = session.MakeRequest(t, req, http.StatusOK)

	return resp
}

func TestPullMerge(t *testing.T) {
	prepareTestEnv(t)
	session := loginUser(t, "user1")
	testRepoFork(t, session, "user2", "repo1", "user1", "repo1")
	testEditFile(t, session, "user1", "repo1", "master", "README.md", "Hello, World (Edited)\n")

	resp := testPullCreate(t, session, "user1", "repo1", "master", "This is a pull title")

	elem := strings.Split(test.RedirectURL(resp), "/")
	assert.EqualValues(t, "pulls", elem[3])
	testPullMerge(t, session, elem[1], elem[2], elem[4], models.MergeStyleMerge)
}

func TestPullRebase(t *testing.T) {
	prepareTestEnv(t)
	session := loginUser(t, "user1")
	testRepoFork(t, session, "user2", "repo1", "user1", "repo1")
	testEditFile(t, session, "user1", "repo1", "master", "README.md", "Hello, World (Edited)\n")

	resp := testPullCreate(t, session, "user1", "repo1", "master", "This is a pull title")

	elem := strings.Split(test.RedirectURL(resp), "/")
	assert.EqualValues(t, "pulls", elem[3])
	testPullMerge(t, session, elem[1], elem[2], elem[4], models.MergeStyleRebase)
}

func TestPullSquash(t *testing.T) {
	prepareTestEnv(t)
	session := loginUser(t, "user1")
	testRepoFork(t, session, "user2", "repo1", "user1", "repo1")
	testEditFile(t, session, "user1", "repo1", "master", "README.md", "Hello, World (Edited)\n")
	testEditFile(t, session, "user1", "repo1", "master", "README.md", "Hello, World (Edited!)\n")

	resp := testPullCreate(t, session, "user1", "repo1", "master", "This is a pull title")

	elem := strings.Split(test.RedirectURL(resp), "/")
	assert.EqualValues(t, "pulls", elem[3])
	testPullMerge(t, session, elem[1], elem[2], elem[4], models.MergeStyleSquash)
}

func TestPullCleanUpAfterMerge(t *testing.T) {
	prepareTestEnv(t)
	session := loginUser(t, "user1")
	testRepoFork(t, session, "user2", "repo1", "user1", "repo1")
	testEditFileToNewBranch(t, session, "user1", "repo1", "master", "feature/test", "README.md", "Hello, World (Edited)\n")

	resp := testPullCreate(t, session, "user1", "repo1", "feature/test", "This is a pull title")

	elem := strings.Split(test.RedirectURL(resp), "/")
	assert.EqualValues(t, "pulls", elem[3])
	testPullMerge(t, session, elem[1], elem[2], elem[4], models.MergeStyleMerge)

	// Check PR branch deletion
	resp = testPullCleanUp(t, session, elem[1], elem[2], elem[4])
	respJSON := struct {
		Redirect string
	}{}
	DecodeJSON(t, resp, &respJSON)

	assert.NotEmpty(t, respJSON.Redirect, "Redirected URL is not found")

	elem = strings.Split(respJSON.Redirect, "/")
	assert.EqualValues(t, "pulls", elem[3])

	// Check branch deletion result
	req := NewRequest(t, "GET", respJSON.Redirect)
	resp = session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	resultMsg := htmlDoc.doc.Find(".ui.message>p").Text()

	assert.EqualValues(t, "user1/feature/test has been deleted.", resultMsg)
}
