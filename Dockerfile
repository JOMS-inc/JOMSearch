FROM golang:1.27

WORKDIR /app

COPY go-services/go.mod go-services/go.sum ./

RUN go mod download

COPY go-services/ .

RUN go build -o server ./cmd/search-api

EXPOSE 8080

CMD ["./server"]
