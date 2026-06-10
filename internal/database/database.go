package database

import (
	"database/sql"
	"fmt"
	"forum/internal/models"
	"os"
	"path/filepath"

	// Blank import: we do not use this package directly in our code.
	// But importing it runs its init() function, which registers the
	// "sqlite3" driver with Go's database/sql package.
	// Without this line, sql.Open("sqlite3", ...) would fail.
	_ "github.com/mattn/go-sqlite3"
)

// Init opens the SQLite database file and initializes the schema.
// It returns a *sql.DB connection pool ready for use.
// The caller (main.go) is responsible for closing it with db.Close().
func Init(dbPath string, schemaPath string) (*sql.DB, error) {
	// Create the database directory if it does not exist yet.
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// sql.Open does not actually connect to the database yet.
	// It just validates the arguments and prepares the connection pool.
	// The first real connection happens on the first query.
	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// SQLite is a file, not a network service — db.Ping() is not needed here.
	// Foreign keys are disabled by default in SQLite.
	// We must enable it explicitly on every new connection.
	// Without this, ON DELETE CASCADE and FK constraints are silently ignored.
	_, err = db.Exec("PRAGMA foreign_keys = ON;")
	if err != nil {
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Read the schema.sql file from disk.
	// os.ReadFile returns the entire file as a []byte slice.
	schema, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema file: %w", err)
	}

	// Execute the schema SQL.
	// db.Exec runs a SQL statement and discards the result rows.
	// Because every statement uses CREATE TABLE IF NOT EXISTS,
	// this is safe to run every time — it will not overwrite existing data.
	_, err = db.Exec(string(schema))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return db, nil

}

func getUserVoteForPost(db *sql.DB, userID, postID int64) int {
	if userID == 0 {
		return 0
	}

	var islike bool
	err := db.QueryRow(
		"SELECT is_like FROM likes WHERE user_id = ? AND post_id = ?",
		userID, postID,
	).Scan(&islike)

	if err != nil {
		return 0
	}

	if islike {
		return 1
	}
	return -1

}

func getUserVoteForComment(db *sql.DB, userID, commentID int64) int {
	if userID == 0 {
		return 0
	}
	var isLike bool
	err := db.QueryRow(
		"SELECT is_like FROM likes WHERE user_id = ? AND comment_id = ?",
		userID, commentID,
	).Scan(&isLike)
	if err != nil {
		return 0
	}
	if isLike {
		return 1
	}
	return -1
}

// -----------------------------------------------------------
// CATEGORIES QUERIES
// -----------------------------------------------------------
// GetAllCategories returns every category row
func GetAllCategories(db *sql.DB) ([]models.Category, error) {
	rows, err := db.Query("SELECT id, name FROM categories ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("failed to query categories: %w", err)
	}

	// defer rows.Close() ensures we release the database connection
	// when this function returns, even if we return early due to an error
	defer rows.Close()

	var categories []models.Category

	// rows.Next() advances the cursor one row at a time.
	// It returns false when there are no more rows or an error occurred.
	for rows.Next() {
		var c models.Category
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			return nil, fmt.Errorf("failed to scan category: %w", err)
		}
		categories = append(categories, c)
	}

	// rows.Err() returns any error that occurred during the iteration.
	// We always need to check this afte a rows.Next() loop.
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("category rows error: %w", err)

	}

	return categories, nil
}

// getCategoriesForPost(db *sql.DB) returns all category names attached to a post.
// This is called for each post when building the post list.
func getCategoriesForPost(db *sql.DB, postID int64) ([]string, error) {
	rows, err := db.Query(`
		SELECT c.name
		FROM categories c
		JOIN post_categories pc ON c.id = pc.category_id
		WHERE pc.post_id = ?
		ORDER BY c.name`,
		postID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query post categories: %w", err)
	}

	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("failed to scan category name: %w", err)
		}
		names = append(names, name)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("category rows error: %w", err)
	}

	return names, nil
}

// -----------------------------------------------------------
// POST QUERIES
// -----------------------------------------------------------

// CreatePost inserts a new post and links it to its categories
// Uses a transaction - either both the post INSERT and the category
// links INSERT succeed together, or neither does.
// This prevents posts existing with no categories attached.
func CreatePost(db *sql.DB, userID int64, title, content, imagePath string, categoryIDs []int64) (int64, error) {
	// Begin a transaction
	// A transaction groups multiple queries into one atomic operation.
	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}

	// defer a rollback. If we return early due to any error,
	// the transaction is rolled back automatically.
	// If we call tx.Commit() successfully, this Rollback becomes a no-op.
	defer tx.Rollback()

	result, err := tx.Exec(
		`INSERT INTO posts (user_id, title, content, image_path)
		 VALUES (?, ?, ?, ?)`,
		userID, title, content, imagePath,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to insert post: %w", err)
	}

	// Get the auto-generated ID of the post we just inserted.
	postID, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get post ID: %w", err)
	}

	// Insert one row into post_categories foe each selected category.
	for _, categoryID := range categoryIDs {
		_, err := tx.Exec(
			"INSERT INTO post_categories (post_id, category_id) VALUES (?, ?)",
			postID, categoryID,
		)
		if err != nil {
			return 0, fmt.Errorf("failed to link category: %w", err)
		}
	}

	// Commit the transaction - makes all changes permanent.
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return postID, nil
}

// GetAllPosts returns all posts ordered newest first.
// Each post includes the author's username and like/dislike counts.
// Categories are fetched seperately per post.
func GetAllPosts(db *sql.DB, userID int64) ([]models.Post, error) {
	rows, err := db.Query(`
		SELECT 
			p.id,
			p.user_id, 
			u.username, 
			p.title,
			p.content,
			p.image_path,
			p.created_at,
			-- COUNT with FILTER counts only rows where condition is true 
			COUNT(CASE WHEN l.is_like = 1 THEN 1 END) AS like_count, 
			COUNT(CASE WHEN l.is_like = 0 THEN 1 END) AS dislike_count
		FROM posts p
		-- JOIN users to get the author's username 
		JOIN users u ON p.user_id = u.id
		-- LEFT JOIN likes so posts with zero ikes still appear 
		LEFT JOIN likes l ON p.id = l.post_id
		GROUP BY p.id 
		ORDER BY p.created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query posts: %w", err)
	}
	defer rows.Close()

	return scanPosts(db, rows, userID)
}

// GetPostByID returns a single post by its ID with full details and comments.
// Returns sql.ErrNoRows if the post does not exist.
func GetPostByID(db *sql.DB, postID, userID int64) (models.Post, []models.Comment, error) {
	var p models.Post

	err := db.QueryRow(`
		SELECT 
			p.id,
			p.user_id,
			u.username,
			p.title,
			p.content,
			p.image_path, 
			p.created_at,
			COUNT(CASE WHEN l.is_like = 1 THEN 1 END) AS like_count,
			COUNT(CASE WHEN l.is_like = 0 THEN 1 END) AS dislike_count
		FROM posts p
		JOIN users u ON p.user_id = u.id
		LEFT JOIN likes l ON p.id = l.post_id
		WHERE p.id = ?
		GROUP BY p.id`,
		postID,
	).Scan(
		&p.ID,
		&p.UserID,
		&p.Username,
		&p.Title,
		&p.Content,
		&p.ImagePath,
		&p.CreatedAt,
		&p.LikeCount,
		&p.DislikeCount,
	)

	if err == sql.ErrNoRows {
		// Return the sentinel error directly so the handler can
		// distinguish "not found" from a real database failure
		return p, nil, sql.ErrNoRows
	}

	if err != nil {
		return p, nil, fmt.Errorf("failed to query post: %w", err)
	}

	// Fetch categories for this post.
	categories, err := getCategoriesForPost(db, p.ID)
	if err != nil {
		return p, nil, fmt.Errorf("failed to get categories: %w", err)
	}
	p.Categories = categories

	// Fetch comments for this post.
	comments, err := GetCommentsByPostID(db, postID, userID)
	if err != nil {
		return p, nil, fmt.Errorf("failed to get comments: %w", err)
	}
	p.UserVote = getUserVoteForPost(db, userID, p.ID)

	return p, comments, nil
}

// -----------------------------------------------------------
// COMMENT QUERIES
// -----------------------------------------------------------

// CreateComment inserts a new comment on a post.
// Returns the new comment's ID
func CreateComment(db *sql.DB, postID, userID int64, content string) (int64, error) {
	result, err := db.Exec(
		`INSERT INTO comments (post_id, user_id, content) VALUES (?, ?, ?)`,
		postID, userID, content,
	)

	if err != nil {
		return 0, fmt.Errorf("failed to insert comment: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get comment ID: %w", err)
	}

	return id, nil
}

// GetCommentsByPostID returns all comments for a given post,
// ordered oldest first so conversation reads top to bottom.
// Each comment includes the author's username and like/dislike counts.
func GetCommentsByPostID(db *sql.DB, postID, userID int64) ([]models.Comment, error) {
	rows, err := db.Query(
		`SELECT 
			c.id,
			c.post_id,
			c.user_id,
			u.username,
			c.content, 
			c.created_at,
			COUNT(CASE WHEN l.is_like = 1 THEN 1 END) AS like_counts,
			COUNT(CASE WHEN l.is_like = 0 THEN 1 END) AS dislike_counts
		FROM comments c
		JOIN users u ON c.user_id = u.id
		-- LEFT JOIN so comments with zero likes still appear
		LEFT JOIN likes l ON c.id = l.comment_id
		WHERE c.post_id = ?
		GROUP BY c.id
		ORDER BY c.created_at ASC`,
		postID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query comments: %w", err)
	}

	defer rows.Close()

	var comments []models.Comment

	for rows.Next() {
		var c models.Comment

		err := rows.Scan(
			&c.ID,
			&c.PostID,
			&c.UserID,
			&c.Username,
			&c.Content,
			&c.CreatedAt,
			&c.LikeCount,
			&c.DislikeCount,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan comment: %w", err)
		}
		c.UserVote = getUserVoteForComment(db, userID, c.ID)

		comments = append(comments, c)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("comments row error: %w", err)
	}

	return comments, nil
}

// -----------------------------------------------------------
// LIKE QUERIES
// -----------------------------------------------------------

// ToggleLike handles the full like/dislike toggle logic for posts and comments.
// targetType must be either "post" or "comment".
// isLike: true = like, false = dislike.
//
// Logic:
//
//			-No existing vote -> INSERT
//	     - Existing vote, same value -> 	DELETE (toggle, off)
//	 	- Existing vote, different value -> UPDATAE (switch direction)
func ToggleLike(db *sql.DB, userID, targetID int64, targetType string, isLike bool) error {
	// Build the WHERE clause dynamically based on whether this is
	// a post like or a comment like.
	var targetColumn string
	if targetType == "post" {
		targetColumn = "post_id"
	} else if targetType == "comment" {
		targetColumn = "comment_id"
	} else {
		return fmt.Errorf("Invalid target type: %s", targetType)
	}

	// Check if like/dislike row already exists for this user + target.
	// We read the current is_like value so we can decide what to do.
	var existingIsLike bool
	var existingID int64

	err := db.QueryRow(
		fmt.Sprintf(
			"SELECT id, is_like FROM likes WHERE user_id = ? AND %s = ?",
			targetColumn,
		),
		userID, targetID,
	).Scan(&existingID, &existingIsLike)

	if err == sql.ErrNoRows {
		// No existing vote - INSERT a new like row.
		// Set the appropriate column and leave the other NULL.
		if targetType == "post" {
			_, err = db.Exec(
				`INSERT INTO likes (user_id, post_id, comment_id, is_like)
				VALUES (?, ?, NULL, ?)`,
				userID, targetID, isLike,
			)
		} else {
			_, err = db.Exec(
				`INSERT INTO likes (user_id, post_id, comment_id, is_like)
				VALUES (?, NULL, ?, ?)`,
				userID, targetID, isLike,
			)
		}
		if err != nil {
			return fmt.Errorf("failed to insert like: %w", err)
		}

		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to query existing like: %w", err)
	}

	// Row exists. Decide: toggle off or switch direction.
	isLikeInt := 0
	if isLike {
		isLikeInt = 1
	}
	existingIsLikeInt := 0
	if existingIsLike {
		existingIsLikeInt = 1
	}

	if isLikeInt == existingIsLikeInt {
		// Same value clicked again - toggle off by deleting the row.
		_, err = db.Exec(
			"DELETE FROM likes WHERE id = ?", existingID,
		)
		if err != nil {
			return fmt.Errorf("failed to delete like: %w", err)
		}
		return nil
	}

	_, err = db.Exec(
		"UPDATE likes SET is_like = ? WHERE id = ?",
		isLike, existingID,
	)
	if err != nil {
		return fmt.Errorf("failed to update like: %w", err)
	}

	return nil
}

// GetUserLikeStatus returns whether the user liked or disliked a target.
// Returns exists bool, isLike bool.
// Used by template to show the correct button state (active/inactive).
func GetUserLikeStatus(db *sql.DB, userID, targetID int64, targetType string) (bool, bool, error) {
	var targetColumn string
	if targetType == "post" {
		targetColumn = "post_id"
	} else if targetType == "comment" {
		targetColumn = "comment_id"
	} else {
		return false, false, fmt.Errorf("Invalid type of target: %s", targetType)
	}

	var isLike bool
	err := db.QueryRow(
		fmt.Sprintf(
			"SELECT is_like FROM likes WHERE user_id = ? AND %s = ?",
			targetColumn,
		), userID, targetID,
	).Scan(&isLike)

	if err == sql.ErrNoRows {
		// User has not voted on this target
		return false, false, nil
	}
	if err != nil {
		return false, false, fmt.Errorf("failed to query like status: %w", err)
	}

	// exists=true, isLike=whatever they voted
	return true, isLike, nil
}

// -----------------------------------------------------------
// FILTER QUERIES
// -----------------------------------------------------------

// GetPostsByCategory returns all posts belonging to a specific category.
// Available to all even to guests.
func GetPostsByCategory(db *sql.DB, categoryID int64, userID int64) ([]models.Post, error) {
	rows, err := db.Query(`
		SELECT 
			p.id,
			p.user_id,
			u.username,
			p.title,
			p.content,
			p.image_path,
			p.created_at,
			COUNT(CASE WHEN l.is_like = 1 THEN 1 END) AS like_count,
			COUNT(CASE WHEN l.is_like = 0 THEN 1 END) AS dislike_count
		FROM posts p
		JOIN users u ON p.user_id = u.id
		-- JOIN post_categories to filter by category
		JOIN post_categories pc ON p.id = pc.post_id
		LEFT JOIN likes l ON p.id = l.post_id
		WHERE pc.category_id = ? 
		GROUP BY p.id 
		ORDER BY p.created_at DESC`,
		categoryID)

	if err != nil {
		return nil, fmt.Errorf("failed to query posts by category: %w", err)
	}

	defer rows.Close()

	return scanPosts(db, rows, userID)
}

// GetPostsByUser returns all posts created by a specific user.
// Used for the "my posts" filter - logged-in users only.
func GetPostsByUser(db *sql.DB, userID int64) ([]models.Post, error) {
	rows, err := db.Query(`
		SELECT 
			p.id,
			p.user_id,
			u.username, 
			p.title,
			p.content,
			p.image_path,
			p.created_at,
			COUNT(CASE WHEN l.is_like = 1 THEN 1 END) AS like_count,
			COUNT(CASE WHEN l.is_like = 0 THEN 1 END) AS dislike_count
		FROM posts p
		JOIN users u ON p.user_id = u.id
		LEFT JOIN likes l ON p.id = l.post_id
		WHERE p.user_id = ?
		GROUP BY p.id
		ORDER BY p.created_at DESC`,
		userID,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to query posts by user: %w", err)
	}

	defer rows.Close()

	return scanPosts(db, rows, userID)
}

// GetPostsLikedByUser returns all posts that a specific user has liked.
// User for the "liked posts" filter - logged-in users only.
func GetPostsLikedByUser(db *sql.DB, userID int64) ([]models.Post, error) {
	rows, err := db.Query(`
		SELECT 
			p.id,
			p.user_id,
			u.username,
			p.title,
			p.content,
			p.image_path,
			p.created_at,
			COUNT(CASE WHEN l.is_like = 1 THEN 1 END) AS like_counts,
			COUNT(CASE WHEN l.is_like = 0 THEN 1 END) AS dislike_counts
		FROM posts p 
		JOIN users u ON p.user_id = u.id
		LEFT JOIN likes l ON p.id = l.post_id
		-- Subquery: only include posts where this user has a like (is_like=1)row
		WHERE p.id IN (
			SELECT post_id FROM likes
			WHERE user_id = ? AND is_like = 1 AND post_id IS NOT NULL
		)
		GROUP BY p.id
		ORDER BY p.created_at DESC`,
		userID,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to query liked posts: %w", err)
	}

	defer rows.Close()

	return scanPosts(db, rows, userID)
}

// scanPosts is a shared helper that scans a *sql.Rows result into []models.Post.
// All four post queries (GetAllPosts, GetPostsByCategory, GetPostsByUser,
// GetPostsLikedByUser) return the same columns in the same order —
// so we extract the scanning logic once here instead of duplicating it four times.
func scanPosts(db *sql.DB, rows *sql.Rows, userID int64) ([]models.Post, error) {
	var posts []models.Post

	for rows.Next() {
		var p models.Post
		err := rows.Scan(
			&p.ID,
			&p.UserID,
			&p.Username,
			&p.Title,
			&p.Content,
			&p.ImagePath,
			&p.CreatedAt,
			&p.LikeCount,
			&p.DislikeCount,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan post: %w", err)
		}

		categories, err := getCategoriesForPost(db, p.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get categories for post %d: %w", p.ID, err)
		}

		p.Categories = categories

		p.UserVote = getUserVoteForPost(db, userID, p.ID)

		posts = append(posts, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("posts rows error: %w", err)
	}

	return posts, nil
}

func DeleteComment(db *sql.DB, commentID, userID int64) error {
	result, err := db.Exec(
		"DELETE FROM comments WHERE id = ? AND user_id = ?",
		commentID, userID,
	)
	if err != nil {
		return fmt.Errorf("failed to delete comment")
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to checl RowsAffected()")
	}

	if rows == 0 {
		return fmt.Errorf("comment not found or not owned by the user")
	}
	return nil

}

// DeletePost deletes any post by ID (moderator/admin action).
func DeletePost(db *sql.DB, postID int64) error {
	result, err := db.Exec("DELETE FROM posts WHERE id = ?", postID)
	if err != nil {
		return fmt.Errorf("failed to delete post: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("post not found")
	}
	return nil
}

// DeleteAnyComment deletes any comment by ID (moderator/admin action).
// Different from DeleteComment which checks ownership.
func DeleteAnyComment(db *sql.DB, commentID int64) error {
	result, err := db.Exec("DELETE FROM comments WHERE id = ?", commentID)
	if err != nil {
		return fmt.Errorf("failed to delete comment: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("comment not found")
	}
	return nil
}

// GetAllUsers returns all users — for the admin panel.
func GetAllUsers(db *sql.DB) ([]models.User, error) {
	rows, err := db.Query(
		"SELECT id, email, username, role, created_at FROM users ORDER BY created_at DESC",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Email, &u.Username, &u.Role, &u.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// SetUserRole updates a user's role. Valid values: "user", "moderator", "admin".
func SetUserRole(db *sql.DB, userID int64, role string) error {
	if role != "user" && role != "moderator" && role != "admin" {
		return fmt.Errorf("invalid role: %s", role)
	}
	_, err := db.Exec("UPDATE users SET role = ? WHERE id = ?", role, userID)
	if err != nil {
		return fmt.Errorf("failed to update role: %w", err)
	}
	return nil
}
