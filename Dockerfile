# Fly's remote builder runs this file (`mise run fly:deploy`); no local Docker is needed.
FROM golang:1.27.1 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# The SQLite driver is pure Go, so the binary is static and needs no C library.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /todo ./cmd/todo

FROM gcr.io/distroless/static-debian12
COPY --from=build /todo /todo
ENTRYPOINT ["/todo"]
CMD ["-addr", ":8080", "-db", "/data/todo.db", "-client-ip-header", "Fly-Client-IP"]
