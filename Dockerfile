# Production image: build the binary, then copy it into a small runtime image.

FROM golang:1.27 AS build

WORKDIR /app

COPY go-services/go.mod go-services/go.sum ./

RUN go mod download

COPY go-services/ .

# modernc.org/sqlite is pure Go, so we can build a static binary without CGO.
RUN CGO_ENABLED=0 go build -o server ./cmd/search-api

FROM alpine:3.22

WORKDIR /app

COPY --from=build /app/server .

EXPOSE 8080

CMD ["./server"]
