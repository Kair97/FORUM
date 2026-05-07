package database

import (
	"database/sql"
	"fmt"
	"forum/internal/models"
	"os"

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

	// sql.Open does not actually connect to the database yet.
	// It just validates the arguments and prepares the connection pool.
	// The first real connection happens on the first query.
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// db.Ping() forces an actual connection to the database.
	// This is where we find out if the file path is valid and accessible.
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// SQLite does not enforce foreign keys by default.
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

// ------------------------
// CATEGORIES QUERIES
// ------------------------
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

// ------------------------
// POST QUERIES
// ------------------------

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
func GetAllPosts(db *sql.DB) ([]models.Post, error) {
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
			return nil, fmt.Errorf("failed to scan the post: %w", err)
		}

		// Fetch categories for this post seperately.
		categories, err := getCategoriesForPost(db, p.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get categories for post %d: %w", p.ID, err)
		}
		p.Categories = categories

		posts = append(posts, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("post rows error: %w", err)
	}

	return posts, nil
}

// GetPostByID returns a single post by its ID with full details.
// Returns sql.ErrNoRows if the post does not exist.
func GetPostByID(db *sql.DB, postID int64) (models.Post, error) {
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
		return p, sql.ErrNoRows
	}

	if err != nil {
		return p, fmt.Errorf("failed to query post: %w", err)
	}

	// Fetch categories for this post.
	categories, err := getCategoriesForPost(db, p.ID)
	if err != nil {
		return p, fmt.Errorf("failed to get categories: %w", err)
	}
	p.Categories = categories

	return p, nil
}
