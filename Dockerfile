# Multi-stage build for the forum Go application.

# Stage 1: Builder
FROM golang:1.25.3-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download 

COPY . . 

RUN CGO_ENABLED=1 GOOS=linux go build -o forum ./cmd/

FROM alpine:3.19

RUN apk add --no-cache ca-certificates sqlite-libs

WORKDIR /app

COPY --from=builder /app/forum .

COPY web/ web/

COPY migrations/ migrations/

RUN mkdir -p database

RUN mkdir -p web/static/uploads

EXPOSE 8080

CMD ["./forum"]