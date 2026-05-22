FROM golang:1.24-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/search-trends ./cmd/search-trends

FROM alpine:3.20

RUN adduser -D -H app
USER app
COPY --from=build /bin/search-trends /bin/search-trends

EXPOSE 8080
ENTRYPOINT ["/bin/search-trends"]
