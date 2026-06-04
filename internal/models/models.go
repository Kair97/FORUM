// internal/models/models.go
// Defines Go structs that map directly to database tables.
// Every database row we read gets scanned into one of these structs.

package models

import "time"

type User struct {
	ID           int64
	Email        string
	Username     string
	PasswordHash string
	Avatar       string
	Role         string
	CreatedAt    time.Time
}

type Session struct {
	ID        int64
	UserID    int64
	Token     string
	ExpiresAt time.Time
}

type Post struct {
	ID           int64
	UserID       int64
	Username     string
	Title        string
	Content      string
	ImagePath    string
	CreatedAt    time.Time
	Categories   []string
	LikeCount    int
	DislikeCount int
	UserVote     int
}

type Category struct {
	ID   int64
	Name string
}

type Comment struct {
	ID           int64
	PostID       int64
	UserID       int64
	Username     string
	Content      string
	CreatedAt    time.Time
	LikeCount    int
	DislikeCount int
	UserVote     int
}

type Like struct {
	ID        int64
	UserID    int64
	PostID    *int64 // pointer because this can be null in database
	CommentID *int64 // same as PostID it can be null
	IsLike    bool
	CreatedAt time.Time
}

type Template struct {
	Posts      []Post
	Post       Post
	Comments   []Comment
	Categories []Category
	UserID     int64
	Username   string
	LoggedIn   bool
	Error      string
	Filter     string
	CategoryID int64
	Users      []User
	Role       string
	// FormEmail and FormUsername let us put the user's typed text
	// back into the form when there is a validation error,
	// so the user does not have to type everything again.
	FormEmail    string
	FormUsername string
}
