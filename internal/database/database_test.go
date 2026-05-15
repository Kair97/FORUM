// internal/database/database_test.go
// Unit tests for database initialization and core SQL queries.
// Each test creates a fresh temporary SQLite database so tests
// are fully isolated and do not touch the real forum.db file.

package database

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// newTestDB creates a fresh temporary SQLite database with the full schema.
// Each call gets its own file-backed database inside t.TempDir().
// Each call creates a completely independent database — no test pollution.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper() // marks this as a helper — failures show the caller's line

	// Use a per-test file instead of ":memory:" because sql.DB may open more
	// than one connection, and separate SQLite in-memory connections do not
	// share tables with each other.
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Init(dbPath, "../../migrations/schema.sql")
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	// Register cleanup — db.Close() is called when the test finishes.
	t.Cleanup(func() { db.Close() })

	return db
}

// seedUser inserts a test user and returns their ID.
// Used as setup for tests that need an existing user.
func seedUser(t *testing.T, db *sql.DB, username, email string) int64 {
	t.Helper()
	result, err := db.Exec(
		`INSERT INTO users (email, username, password_hash, role)
		 VALUES (?, ?, ?, ?)`,
		email, username, "$2a$12$fakehashfortesting", "user",
	)
	if err != nil {
		t.Fatalf("seedUser failed: %v", err)
	}
	id, _ := result.LastInsertId()
	return id
}

// ─────────────────────────────────────────────
// DATABASE INITIALIZATION TESTS
// ─────────────────────────────────────────────

// TestInitCreatesAllTables verifies that Init creates every required table.
func TestInitCreatesAllTables(t *testing.T) {
	db := newTestDB(t)

	// These are the tables our schema.sql must create.
	requiredTables := []string{
		"users",
		"sessions",
		"posts",
		"categories",
		"post_categories",
		"comments",
		"likes",
	}

	for _, table := range requiredTables {
		t.Run("table_"+table, func(t *testing.T) {
			// sqlite_master contains metadata about all tables.
			// If this query returns a row, the table exists.
			var name string
			err := db.QueryRow(
				"SELECT name FROM sqlite_master WHERE type='table' AND name=?",
				table,
			).Scan(&name)

			if err == sql.ErrNoRows {
				t.Errorf("table %q was not created by Init", table)
			} else if err != nil {
				t.Errorf("error checking table %q: %v", table, err)
			}
		})
	}
}

// TestInitSeedsCategories verifies that default categories are inserted.
func TestInitSeedsCategories(t *testing.T) {
	db := newTestDB(t)

	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM categories").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count categories: %v", err)
	}

	if count == 0 {
		t.Error("Init did not seed any categories")
	}
}

// ─────────────────────────────────────────────
// POST TESTS
// ─────────────────────────────────────────────

// TestCreatePost verifies that a post is inserted and returns a valid ID.
func TestCreatePost(t *testing.T) {
	db := newTestDB(t)
	userID := seedUser(t, db, "alice", "alice@test.com")

	// Get a real category ID from the seeded categories.
	var categoryID int64
	err := db.QueryRow("SELECT id FROM categories LIMIT 1").Scan(&categoryID)
	if err != nil {
		t.Fatalf("failed to get category: %v", err)
	}

	postID, err := CreatePost(db, userID, "Test Title", "Test Content", "", []int64{categoryID})
	if err != nil {
		t.Fatalf("CreatePost returned error: %v", err)
	}

	if postID <= 0 {
		t.Errorf("CreatePost returned invalid ID: %d", postID)
	}
}

// TestCreatePostRequiresCategory verifies the transaction rollback behavior.
// A post with an invalid category ID should fail entirely.
func TestCreatePostRequiresCategory(t *testing.T) {
	db := newTestDB(t)
	userID := seedUser(t, db, "bob", "bob@test.com")

	// Category ID 99999 does not exist — foreign key should reject this.
	_, err := CreatePost(db, userID, "Title", "Content", "", []int64{99999})
	if err == nil {
		t.Error("CreatePost should have failed with non-existent category ID")
	}
}

// TestGetAllPosts verifies that inserted posts are returned.
func TestGetAllPosts(t *testing.T) {
	db := newTestDB(t)
	userID := seedUser(t, db, "alice", "alice@test.com")

	var categoryID int64
	db.QueryRow("SELECT id FROM categories LIMIT 1").Scan(&categoryID)

	// Insert two posts.
	_, err := CreatePost(db, userID, "First Post", "Content 1", "", []int64{categoryID})
	if err != nil {
		t.Fatalf("failed to create first post: %v", err)
	}
	_, err = CreatePost(db, userID, "Second Post", "Content 2", "", []int64{categoryID})
	if err != nil {
		t.Fatalf("failed to create second post: %v", err)
	}

	// GetAllPosts should return both.
	posts, err := GetAllPosts(db, 0) // 0 = guest user
	if err != nil {
		t.Fatalf("GetAllPosts returned error: %v", err)
	}

	if len(posts) != 2 {
		t.Errorf("GetAllPosts returned %d posts, expected 2", len(posts))
	}
}

// TestGetPostByID verifies correct post retrieval by ID.
func TestGetPostByID(t *testing.T) {
	db := newTestDB(t)
	userID := seedUser(t, db, "alice", "alice@test.com")

	var categoryID int64
	db.QueryRow("SELECT id FROM categories LIMIT 1").Scan(&categoryID)

	postID, err := CreatePost(db, userID, "My Post", "My Content", "", []int64{categoryID})
	if err != nil {
		t.Fatalf("failed to create post: %v", err)
	}

	post, _, err := GetPostByID(db, postID, 0)
	if err != nil {
		t.Fatalf("GetPostByID returned error: %v", err)
	}

	if post.Title != "My Post" {
		t.Errorf("expected title 'My Post', got %q", post.Title)
	}
	if post.Content != "My Content" {
		t.Errorf("expected content 'My Content', got %q", post.Content)
	}
}

// TestGetPostByIDNotFound verifies that a missing post returns ErrNoRows.
func TestGetPostByIDNotFound(t *testing.T) {
	db := newTestDB(t)

	_, _, err := GetPostByID(db, 99999, 0)
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows for missing post, got: %v", err)
	}
}

// ─────────────────────────────────────────────
// LIKE TOGGLE TESTS
// ─────────────────────────────────────────────

// TestToggleLikeInsert verifies that a new like is inserted correctly.
func TestToggleLikeInsert(t *testing.T) {
	db := newTestDB(t)
	userID := seedUser(t, db, "alice", "alice@test.com")

	var categoryID int64
	db.QueryRow("SELECT id FROM categories LIMIT 1").Scan(&categoryID)
	postID, _ := CreatePost(db, userID, "Post", "Content", "", []int64{categoryID})

	// Like the post.
	err := ToggleLike(db, userID, postID, "post", true)
	if err != nil {
		t.Fatalf("ToggleLike returned error: %v", err)
	}

	// Verify the like exists in the database.
	var count int
	db.QueryRow(
		"SELECT COUNT(*) FROM likes WHERE user_id=? AND post_id=? AND is_like=1",
		userID, postID,
	).Scan(&count)

	if count != 1 {
		t.Errorf("expected 1 like row, found %d", count)
	}
}

// TestToggleLikeRemove verifies that liking twice removes the like (toggle off).
func TestToggleLikeRemove(t *testing.T) {
	db := newTestDB(t)
	userID := seedUser(t, db, "alice", "alice@test.com")

	var categoryID int64
	db.QueryRow("SELECT id FROM categories LIMIT 1").Scan(&categoryID)
	postID, _ := CreatePost(db, userID, "Post", "Content", "", []int64{categoryID})

	// Like it once.
	ToggleLike(db, userID, postID, "post", true)

	// Like it again — should remove the like.
	err := ToggleLike(db, userID, postID, "post", true)
	if err != nil {
		t.Fatalf("second ToggleLike returned error: %v", err)
	}

	// Row should be gone.
	var count int
	db.QueryRow(
		"SELECT COUNT(*) FROM likes WHERE user_id=? AND post_id=?",
		userID, postID,
	).Scan(&count)

	if count != 0 {
		t.Errorf("expected 0 like rows after toggle off, found %d", count)
	}
}

// TestToggleLikeSwitch verifies switching from like to dislike.
func TestToggleLikeSwitch(t *testing.T) {
	db := newTestDB(t)
	userID := seedUser(t, db, "alice", "alice@test.com")

	var categoryID int64
	db.QueryRow("SELECT id FROM categories LIMIT 1").Scan(&categoryID)
	postID, _ := CreatePost(db, userID, "Post", "Content", "", []int64{categoryID})

	// Like first.
	ToggleLike(db, userID, postID, "post", true)

	// Then dislike — should switch.
	err := ToggleLike(db, userID, postID, "post", false)
	if err != nil {
		t.Fatalf("switch ToggleLike returned error: %v", err)
	}

	// Should have exactly one row with is_like=0 (dislike).
	var isLike bool
	err = db.QueryRow(
		"SELECT is_like FROM likes WHERE user_id=? AND post_id=?",
		userID, postID,
	).Scan(&isLike)

	if err != nil {
		t.Fatalf("failed to query like after switch: %v", err)
	}
	if isLike {
		t.Error("expected is_like=false after switching to dislike, got true")
	}
}

// ─────────────────────────────────────────────
// COMMENT TESTS
// ─────────────────────────────────────────────

// TestCreateComment verifies comment insertion.
func TestCreateComment(t *testing.T) {
	db := newTestDB(t)
	userID := seedUser(t, db, "alice", "alice@test.com")

	var categoryID int64
	db.QueryRow("SELECT id FROM categories LIMIT 1").Scan(&categoryID)
	postID, _ := CreatePost(db, userID, "Post", "Content", "", []int64{categoryID})

	commentID, err := CreateComment(db, postID, userID, "Great post!")
	if err != nil {
		t.Fatalf("CreateComment returned error: %v", err)
	}
	if commentID <= 0 {
		t.Errorf("CreateComment returned invalid ID: %d", commentID)
	}
}

// TestDeleteComment verifies a user can delete their own comment.
func TestDeleteComment(t *testing.T) {
	db := newTestDB(t)
	userID := seedUser(t, db, "alice", "alice@test.com")

	var categoryID int64
	db.QueryRow("SELECT id FROM categories LIMIT 1").Scan(&categoryID)
	postID, _ := CreatePost(db, userID, "Post", "Content", "", []int64{categoryID})
	commentID, _ := CreateComment(db, postID, userID, "My comment")

	// Delete own comment — should succeed.
	err := DeleteComment(db, commentID, userID)
	if err != nil {
		t.Fatalf("DeleteComment returned error: %v", err)
	}

	// Verify it is gone.
	var count int
	db.QueryRow("SELECT COUNT(*) FROM comments WHERE id=?", commentID).Scan(&count)
	if count != 0 {
		t.Error("comment still exists after DeleteComment")
	}
}

// TestDeleteCommentWrongUser verifies a user cannot delete another user's comment.
func TestDeleteCommentWrongUser(t *testing.T) {
	db := newTestDB(t)
	aliceID := seedUser(t, db, "alice", "alice@test.com")
	bobID := seedUser(t, db, "bob", "bob@test.com")

	var categoryID int64
	db.QueryRow("SELECT id FROM categories LIMIT 1").Scan(&categoryID)
	postID, _ := CreatePost(db, aliceID, "Post", "Content", "", []int64{categoryID})
	commentID, _ := CreateComment(db, postID, aliceID, "Alice's comment")

	// Bob tries to delete Alice's comment — should fail.
	err := DeleteComment(db, commentID, bobID)
	if err == nil {
		t.Error("DeleteComment should have failed when wrong user tries to delete")
	}

	// Comment should still exist.
	var count int
	db.QueryRow("SELECT COUNT(*) FROM comments WHERE id=?", commentID).Scan(&count)
	if count != 1 {
		t.Error("comment was deleted by wrong user — ownership check failed")
	}
}

// TestGetCommentsByPostID verifies comments are returned for the correct post.
func TestGetCommentsByPostID(t *testing.T) {
	db := newTestDB(t)
	userID := seedUser(t, db, "alice", "alice@test.com")

	var categoryID int64
	db.QueryRow("SELECT id FROM categories LIMIT 1").Scan(&categoryID)
	postID, _ := CreatePost(db, userID, "Post", "Content", "", []int64{categoryID})

	// Insert three comments on the post.
	CreateComment(db, postID, userID, "Comment 1")
	CreateComment(db, postID, userID, "Comment 2")
	CreateComment(db, postID, userID, "Comment 3")

	comments, err := GetCommentsByPostID(db, postID, 0)
	if err != nil {
		t.Fatalf("GetCommentsByPostID returned error: %v", err)
	}

	if len(comments) != 3 {
		t.Errorf("expected 3 comments, got %d", len(comments))
	}
}

// TestGetCommentsByPostIDEmpty verifies empty slice returned for post with no comments.
func TestGetCommentsByPostIDEmpty(t *testing.T) {
	db := newTestDB(t)
	userID := seedUser(t, db, "alice", "alice@test.com")

	var categoryID int64
	db.QueryRow("SELECT id FROM categories LIMIT 1").Scan(&categoryID)
	postID, _ := CreatePost(db, userID, "Post", "Content", "", []int64{categoryID})

	comments, err := GetCommentsByPostID(db, postID, 0)
	if err != nil {
		t.Fatalf("GetCommentsByPostID returned error: %v", err)
	}

	// Should return empty slice, not nil and not error.
	if comments == nil {
		// nil is acceptable here — range over nil slice works fine in Go
		// but we document this expectation explicitly
		t.Log("GetCommentsByPostID returned nil for empty result — this is acceptable")
	}

	if len(comments) != 0 {
		t.Errorf("expected 0 comments, got %d", len(comments))
	}
}

// ─────────────────────────────────────────────
// FILTER TESTS
// ─────────────────────────────────────────────

// TestGetPostsByUser verifies filter returns only the correct user's posts.
func TestGetPostsByUser(t *testing.T) {
	db := newTestDB(t)
	aliceID := seedUser(t, db, "alice", "alice@test.com")
	bobID := seedUser(t, db, "bob", "bob@test.com")

	var categoryID int64
	db.QueryRow("SELECT id FROM categories LIMIT 1").Scan(&categoryID)

	// Alice creates 2 posts, Bob creates 1.
	CreatePost(db, aliceID, "Alice Post 1", "Content", "", []int64{categoryID})
	CreatePost(db, aliceID, "Alice Post 2", "Content", "", []int64{categoryID})
	CreatePost(db, bobID, "Bob Post", "Content", "", []int64{categoryID})

	// Filter by Alice — should only get 2 posts.
	posts, err := GetPostsByUser(db, aliceID)
	if err != nil {
		t.Fatalf("GetPostsByUser returned error: %v", err)
	}

	if len(posts) != 2 {
		t.Errorf("expected 2 posts for alice, got %d", len(posts))
	}

	// Verify all returned posts belong to Alice.
	for _, p := range posts {
		if p.UserID != aliceID {
			t.Errorf("post %d belongs to user %d, expected alice (%d)",
				p.ID, p.UserID, aliceID)
		}
	}
}

// TestGetPostsLikedByUser verifies filter returns only liked posts.
func TestGetPostsLikedByUser(t *testing.T) {
	db := newTestDB(t)
	aliceID := seedUser(t, db, "alice", "alice@test.com")
	bobID := seedUser(t, db, "bob", "bob@test.com")

	var categoryID int64
	db.QueryRow("SELECT id FROM categories LIMIT 1").Scan(&categoryID)

	post1ID, _ := CreatePost(db, bobID, "Post 1", "Content", "", []int64{categoryID})
	post2ID, _ := CreatePost(db, bobID, "Post 2", "Content", "", []int64{categoryID})
	post3ID, _ := CreatePost(db, bobID, "Post 3", "Content", "", []int64{categoryID})

	// Alice likes post 1 and post 3, not post 2.
	ToggleLike(db, aliceID, post1ID, "post", true)
	ToggleLike(db, aliceID, post3ID, "post", true)

	// Suppress unused variable warning
	_ = post2ID

	posts, err := GetPostsLikedByUser(db, aliceID)
	if err != nil {
		t.Fatalf("GetPostsLikedByUser returned error: %v", err)
	}

	if len(posts) != 2 {
		t.Errorf("expected 2 liked posts, got %d", len(posts))
	}
}

// TestGetPostsByCategory verifies category filter returns correct posts.
func TestGetPostsByCategory(t *testing.T) {
	db := newTestDB(t)
	userID := seedUser(t, db, "alice", "alice@test.com")

	// Get two different category IDs.
	var cat1ID, cat2ID int64
	rows, err := db.Query("SELECT id FROM categories LIMIT 2")
	if err != nil {
		t.Fatalf("failed to query categories: %v", err)
	}
	rows.Next()
	rows.Scan(&cat1ID)
	rows.Next()
	rows.Scan(&cat2ID)
	if err := rows.Close(); err != nil {
		t.Fatalf("failed to close category rows: %v", err)
	}

	// Create posts in different categories.
	if _, err := CreatePost(db, userID, "Cat1 Post 1", "Content", "", []int64{cat1ID}); err != nil {
		t.Fatalf("failed to create first category 1 post: %v", err)
	}
	if _, err := CreatePost(db, userID, "Cat1 Post 2", "Content", "", []int64{cat1ID}); err != nil {
		t.Fatalf("failed to create second category 1 post: %v", err)
	}
	if _, err := CreatePost(db, userID, "Cat2 Post", "Content", "", []int64{cat2ID}); err != nil {
		t.Fatalf("failed to create category 2 post: %v", err)
	}

	posts, err := GetPostsByCategory(db, cat1ID, 0)
	if err != nil {
		t.Fatalf("GetPostsByCategory returned error: %v", err)
	}

	if len(posts) != 2 {
		t.Errorf("expected 2 posts in category 1, got %d", len(posts))
	}
}

// ─────────────────────────────────────────────
// USER VOTE STATE TESTS
// ─────────────────────────────────────────────

// TestUserVoteStateOnPost verifies UserVote is populated correctly.
func TestUserVoteStateOnPost(t *testing.T) {
	db := newTestDB(t)
	userID := seedUser(t, db, "alice", "alice@test.com")

	var categoryID int64
	db.QueryRow("SELECT id FROM categories LIMIT 1").Scan(&categoryID)
	postID, _ := CreatePost(db, userID, "Post", "Content", "", []int64{categoryID})

	// Before voting — UserVote should be 0.
	post, _, err := GetPostByID(db, postID, userID)
	if err != nil {
		t.Fatalf("GetPostByID failed: %v", err)
	}
	if post.UserVote != 0 {
		t.Errorf("expected UserVote=0 before voting, got %d", post.UserVote)
	}

	// Like the post.
	ToggleLike(db, userID, postID, "post", true)

	// After liking — UserVote should be 1.
	post, _, err = GetPostByID(db, postID, userID)
	if err != nil {
		t.Fatalf("GetPostByID failed after like: %v", err)
	}
	if post.UserVote != 1 {
		t.Errorf("expected UserVote=1 after liking, got %d", post.UserVote)
	}

	// Dislike the post (switch direction).
	ToggleLike(db, userID, postID, "post", false)

	// After disliking — UserVote should be -1.
	post, _, err = GetPostByID(db, postID, userID)
	if err != nil {
		t.Fatalf("GetPostByID failed after dislike: %v", err)
	}
	if post.UserVote != -1 {
		t.Errorf("expected UserVote=-1 after disliking, got %d", post.UserVote)
	}
}

// TestGetUserLikeStatus tests the standalone vote status helper.
func TestGetUserLikeStatus(t *testing.T) {
	db := newTestDB(t)
	userID := seedUser(t, db, "alice", "alice@test.com")

	var categoryID int64
	db.QueryRow("SELECT id FROM categories LIMIT 1").Scan(&categoryID)
	postID, _ := CreatePost(db, userID, "Post", "Content", "", []int64{categoryID})

	// No vote yet.
	exists, _, err := GetUserLikeStatus(db, userID, postID, "post")
	if err != nil {
		t.Fatalf("GetUserLikeStatus failed: %v", err)
	}
	if exists {
		t.Error("expected no vote before liking")
	}

	// Like the post.
	ToggleLike(db, userID, postID, "post", true)

	exists, isLike, err := GetUserLikeStatus(db, userID, postID, "post")
	if err != nil {
		t.Fatalf("GetUserLikeStatus failed after like: %v", err)
	}
	if !exists {
		t.Error("expected vote to exist after liking")
	}
	if !isLike {
		t.Error("expected isLike=true after liking")
	}
}
