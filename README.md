# Forum

A full-stack web forum application built in Go — no frameworks, no shortcuts.

![Go](https://img.shields.io/badge/Go-1.22-00ADD8?logo=go)
![SQLite](https://img.shields.io/badge/SQLite-3-003B57?logo=sqlite)
![Docker](https://img.shields.io/badge/Docker-containerized-2496ED?logo=docker)

---

## Features

- User registration and login with secure cookie-based sessions
- Create posts with one or more categories
- Comment on posts
- Like and dislike posts and comments (toggle behavior)
- Filter posts by category, by posts you created, or by posts you liked
- Moderator and admin roles with content moderation tools
- Image upload on posts (JPEG, PNG, GIF, WebP — max 10MB)
- Admin panel for managing user roles
- Fully responsive dark-themed UI — pure HTML, CSS, Go templates
- Docker containerization with multi-stage build

---

## Tech Stack

| Layer | Technology |
|---|---|
| Language | Go 1.22 |
| Database | SQLite 3 (via `github.com/mattn/go-sqlite3`) |
| Password hashing | bcrypt (via `golang.org/x/crypto/bcrypt`) |
| Session tokens | UUID v4 (via `github.com/gofrs/uuid`) |
| Templates | Go `html/template` |
| Containerization | Docker (multi-stage build) |
| Frontend | Pure HTML + CSS (no frameworks) |

---

## Project Structure

```
forum/
├── cmd/
│   └── main.go                 # Entry point — server startup and route registration
├── internal/
│   ├── auth/
│   │   └── auth.go             # bcrypt hashing, UUID session management
│   ├── database/
│   │   └── database.go         # SQLite connection, schema init, all SQL queries
│   ├── handlers/
│   │   ├── auth.go             # Register, login, logout handlers
│   │   ├── posts.go            # Post list, single post, create post handlers
│   │   ├── comments.go         # Comment creation and deletion handlers
│   │   ├── likes.go            # Like/dislike toggle handler
│   │   └── moderation.go       # Moderator/admin action handlers
│   ├── middleware/
│   │   └── middleware.go       # RequireAuth, OptionalAuth, RequireModerator, RequireAdmin
│   ├── models/
│   │   └── models.go           # Go structs mapping to database tables
│   └── utils/
│       └── utils.go            # Template renderer, image upload, context helpers
├── migrations/
│   └── schema.sql              # Full database schema — all CREATE TABLE statements
├── web/
│   ├── static/
│   │   ├── css/style.css       # Application styles
│   │   └── uploads/            # User-uploaded images (served as static files)
│   └── templates/              # Go HTML templates
│       ├── base.html
│       ├── index.html
│       ├── post.html
│       ├── create-post.html
│       ├── login.html
│       ├── register.html
│       └── admin.html
├── database/
│   └── forum.db                # SQLite database file (created on first run)
├── Dockerfile                  # Multi-stage Docker build
├── docker-compose.yml          # Docker Compose configuration
├── build.sh                    # One-command build and run script
├── go.mod
└── go.sum
```

---

## Getting Started

### Run with Docker (recommended)

```bash
git clone <your-repo-url>
cd forum
chmod +x build.sh
./build.sh
```

The forum will be available at `http://localhost:8080`

### Run locally (requires Go 1.22+ and gcc)

```bash
git clone <your-repo-url>
cd forum
go mod download
go run cmd/main.go
```

### Docker commands

```bash
# Build the image
docker build -t forum .

# Run the container
docker run -d \
  --name forum-app \
  -p 8080:8080 \
  -v "$(pwd)/database:/app/database" \
  -v "$(pwd)/web/static/uploads:/app/web/static/uploads" \
  forum

# View logs
docker logs forum-app

# Stop the container
docker stop forum-app

# List images
docker images

# List containers
docker ps -a
```

---

## Database Schema

```
users           — registered accounts (email, username, bcrypt hash, role)
sessions        — active login sessions (UUID token, expiry)
posts           — forum posts (title, content, optional image)
categories      — post categories (seeded with 7 defaults)
post_categories — many-to-many join between posts and categories
comments        — comments on posts
likes           — likes and dislikes on posts and comments
```

All foreign keys use `ON DELETE CASCADE` — deleting a user removes all their content automatically.

---

## Authentication

- Passwords hashed with **bcrypt** (cost factor 12) — never stored in plain text
- Sessions identified by **UUID v4** tokens stored in `HttpOnly`, `SameSite=Lax` cookies
- Session expiry enforced in both the cookie and the database
- Every protected route validated through middleware before the handler runs
- Generic error messages on login failure — prevents user enumeration attacks

---

## User Roles

| Role | Permissions |
|---|---|
| Guest | View posts, comments, like/dislike counts |
| User | + Create posts and comments, like/dislike, filter |
| Moderator | + Delete any post or comment |
| Admin | + Promote users to moderator, access admin panel |

---

## API Routes

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/` | Optional | Homepage with post list and filters |
| GET | `/post?id=N` | Optional | Single post with comments |
| GET | `/post/create` | Required | Create post form |
| POST | `/post/create` | Required | Submit new post |
| POST | `/comment/create` | Required | Submit new comment |
| POST | `/comment/delete` | Required | Delete own comment |
| POST | `/like` | Required | Toggle like or dislike |
| GET | `/register` | None | Registration form |
| POST | `/register` | None | Submit registration |
| GET | `/login` | None | Login form |
| POST | `/login` | None | Submit login |
| POST | `/logout` | None | Logout and clear session |
| GET | `/admin` | Admin | Admin panel |
| POST | `/admin/set-role` | Admin | Change user role |
| POST | `/mod/delete-post` | Moderator | Delete any post |
| POST | `/mod/delete-comment` | Moderator | Delete any comment |

---

## Filtering

| Filter | URL | Access |
|---|---|---|
| All posts | `/` | Everyone |
| By category | `/?category_id=N` | Everyone |
| My posts | `/?filter=created` | Logged-in users |
| Liked posts | `/?filter=liked` | Logged-in users |

---

## Security

- **SQL injection** — all queries use `?` parameterized placeholders, never string concatenation
- **XSS** — Go's `html/template` automatically escapes all user content
- **CSRF** — `SameSite=Lax` cookies block cross-site POST requests
- **Password storage** — bcrypt with cost 12, random salt per hash
- **Session security** — `HttpOnly` flag prevents JavaScript cookie access
- **File upload** — MIME type validated from file content bytes (not extension), UUID filenames prevent path traversal
- **Open redirect** — redirect targets validated to start with `/`
- **Role enforcement** — checked against database on every request, not from cookie or context alone

---

## Running Tests

```bash
go test ./...
```

Tests use an in-memory SQLite database — no real data is touched.

```bash
go test ./... -v        # verbose output with individual test names
go test ./internal/auth/...      # auth tests only
go test ./internal/database/...  # database tests only
```

---

## Making Yourself Admin

After registering your account:

```bash
sqlite3 database/forum.db "UPDATE users SET role='admin' WHERE username='your_username';"
```

---

## Allowed Packages

Per project requirements, only these external packages are used:

- `github.com/mattn/go-sqlite3` — SQLite driver
- `golang.org/x/crypto/bcrypt` — password hashing
- `github.com/gofrs/uuid` — UUID session tokens

All other functionality uses the Go standard library only.

---

## License

MIT
