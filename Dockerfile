# Stage 1: Build the Go binary
FROM golang:1.25.3-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 GOOS=linux go build -o forum ./cmd/

# Stage 2: Minimal runtime image
FROM alpine:3.19

RUN apk add --no-cache ca-certificates sqlite-libs

WORKDIR /app

# Copy the compiled binary
COPY --from=builder /app/forum .

# Copy only templates and CSS — not uploads (those live in the volume)
COPY web/templates/ web/templates/
COPY web/static/css/ web/static/css/
COPY web/static/favicon.svg web/static/favicon.svg

# Copy database migrations
COPY migrations/ migrations/

# Create volume directories so the app works even without Docker volume mounts
RUN mkdir -p volume/database volume/uploaded_imgs

EXPOSE 8080

CMD ["./forum"]
