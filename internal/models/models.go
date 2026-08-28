// Package models holds the plain data structs shared by the downstream
// service clients, the dataloaders, and the GraphQL resolvers. They are
// bound to the generated GraphQL types via gqlgen's autobind.
package models

// User mirrors the data returned by the User Service.
type User struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Avatar string `json:"avatar"`
}

// Post mirrors the data returned by the Post Service. UserID is not part of
// the public GraphQL schema; it is what the Post.user resolver batches on.
type Post struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	CreatedAt string `json:"createdAt"`
	UserID    string `json:"userId"`
}

// Comment mirrors the data returned by the Comment Service. PostID and
// AuthorID are not part of the public GraphQL schema; they are what the
// Comment.post and Comment.author resolvers batch on.
type Comment struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	CreatedAt string `json:"createdAt"`
	PostID    string `json:"postId"`
	AuthorID  string `json:"authorId"`
}
