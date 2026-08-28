// Package mocks contains fixed, cross-linked seed data shared by the three
// mock HTTP services (userservice, postservice, commentservice), so that a
// user id, post id, and comment id fetched independently always resolve to
// a consistent, related object graph.
package mocks

import "graphql-gateway/internal/models"

// Users is the seed data served by the mock User Service.
var Users = map[string]*models.User{
	"1": {ID: "1", Name: "Ada Lovelace", Email: "ada@example.com", Avatar: "https://example.com/avatars/1.png"},
	"2": {ID: "2", Name: "Alan Turing", Email: "alan@example.com", Avatar: "https://example.com/avatars/2.png"},
	"3": {ID: "3", Name: "Grace Hopper", Email: "grace@example.com", Avatar: "https://example.com/avatars/3.png"},
}

// Posts is the seed data served by the mock Post Service, keyed by post ID.
var Posts = map[string]*models.Post{
	"101": {ID: "101", Title: "Analytical Engines", Content: "Notes on the analytical engine.", CreatedAt: "2026-01-05T10:00:00Z", UserID: "1"},
	"102": {ID: "102", Title: "On Computable Numbers", Content: "A paper on the halting problem.", CreatedAt: "2026-01-10T09:30:00Z", UserID: "2"},
	"103": {ID: "103", Title: "Compilers 101", Content: "How the first compiler came to be.", CreatedAt: "2026-01-12T14:15:00Z", UserID: "3"},
	"104": {ID: "104", Title: "Debugging Origins", Content: "The story of the first literal bug.", CreatedAt: "2026-01-20T08:00:00Z", UserID: "3"},
}

// Comments is the seed data served by the mock Comment Service, keyed by comment ID.
var Comments = map[string]*models.Comment{
	"1001": {ID: "1001", Text: "Fascinating read!", CreatedAt: "2026-01-06T11:00:00Z", PostID: "101", AuthorID: "2"},
	"1002": {ID: "1002", Text: "Ahead of its time.", CreatedAt: "2026-01-06T12:00:00Z", PostID: "101", AuthorID: "3"},
	"1003": {ID: "1003", Text: "Still relevant today.", CreatedAt: "2026-01-11T09:00:00Z", PostID: "102", AuthorID: "1"},
	"1004": {ID: "1004", Text: "Great overview.", CreatedAt: "2026-01-13T10:00:00Z", PostID: "103", AuthorID: "1"},
	"1005": {ID: "1005", Text: "Love the history here.", CreatedAt: "2026-01-13T11:00:00Z", PostID: "103", AuthorID: "2"},
}

// PostsByUser indexes Posts by their author's user ID.
func PostsByUser(userID string) []*models.Post {
	var out []*models.Post
	for _, p := range Posts {
		if p.UserID == userID {
			out = append(out, p)
		}
	}
	return out
}

// CommentsByPost indexes Comments by the post they're attached to.
func CommentsByPost(postID string) []*models.Comment {
	var out []*models.Comment
	for _, c := range Comments {
		if c.PostID == postID {
			out = append(out, c)
		}
	}
	return out
}
