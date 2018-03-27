// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"

	"code.gitea.io/gitea/modules/indexer"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

// issueIndexerUpdateQueue queue of issue ids to be updated
var issueIndexerUpdateQueue chan int64

// InitIssueIndexer initialize issue indexer
func InitIssueIndexer() {
	indexer.InitIssueIndexer(populateIssueIndexer)
	issueIndexerUpdateQueue = make(chan int64, setting.Indexer.UpdateQueueLength)
	go processIssueIndexerUpdateQueue()
}

// populateIssueIndexer populate the issue indexer with issue data
func populateIssueIndexer() error {
	batch := indexer.IssueIndexerBatch()
	for page := 1; ; page++ {
		repos, _, err := SearchRepositoryByName(&SearchRepoOptions{
			Page:        page,
			PageSize:    RepositoryListDefaultPageSize,
			OrderBy:     SearchOrderByID,
			Private:     true,
			Collaborate: util.OptionalBoolFalse,
		})
		if err != nil {
			return fmt.Errorf("Repositories: %v", err)
		}
		if len(repos) == 0 {
			return batch.Flush()
		}
		for _, repo := range repos {
			issues, err := Issues(&IssuesOptions{
				RepoIDs:  []int64{repo.ID},
				IsClosed: util.OptionalBoolNone,
				IsPull:   util.OptionalBoolNone,
			})
			if err != nil {
				return err
			}
			if err = IssueList(issues).LoadComments(); err != nil {
				return err
			}
			for _, issue := range issues {
				if err := issue.update().AddToFlushingBatch(batch); err != nil {
					return err
				}
			}
		}
	}
}

func processIssueIndexerUpdateQueue() {
	batch := indexer.IssueIndexerBatch()
	for {
		var issueID int64
		select {
		case issueID = <-issueIndexerUpdateQueue:
		default:
			// flush whatever updates we currently have, since we
			// might have to wait a while
			if err := batch.Flush(); err != nil {
				log.Error(4, "IssueIndexer: %v", err)
			}
			issueID = <-issueIndexerUpdateQueue
		}
		issue, err := GetIssueByID(issueID)
		if err != nil {
			log.Error(4, "GetIssueByID: %v", err)
		} else if err = issue.update().AddToFlushingBatch(batch); err != nil {
			log.Error(4, "IssueIndexer: %v", err)
		}
	}
}

func (issue *Issue) update() indexer.IssueIndexerUpdate {
	comments := make([]string, 0, 5)
	for _, comment := range issue.Comments {
		if comment.Type == CommentTypeComment {
			comments = append(comments, comment.Content)
		}
	}
	return indexer.IssueIndexerUpdate{
		IssueID: issue.ID,
		Data: &indexer.IssueIndexerData{
			RepoID:   issue.RepoID,
			Title:    issue.Title,
			Content:  issue.Content,
			Comments: comments,
		},
	}
}

// updateNeededCols whether a change to the specified columns requires updating
// the issue indexer
func updateNeededCols(cols []string) bool {
	for _, col := range cols {
		switch col {
		case "name", "content":
			return true
		}
	}
	return false
}

// UpdateIssueIndexerCols update an issue in the issue indexer, given changes
// to the specified columns
func UpdateIssueIndexerCols(issueID int64, cols ...string) {
	if updateNeededCols(cols) {
		UpdateIssueIndexer(issueID)
	}
}

// UpdateIssueIndexer add/update an issue to the issue indexer
func UpdateIssueIndexer(issueID int64) {
	select {
	case issueIndexerUpdateQueue <- issueID:
	default:
		go func() {
			issueIndexerUpdateQueue <- issueID
		}()
	}
}
