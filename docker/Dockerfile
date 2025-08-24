# --- build ---
FROM golang:1.23-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /out/app ./cmd/app

# --- run ---
FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/app /app/app
COPY cmd/app/static /app/static/
EXPOSE 8081
ENTRYPOINT ["/app/app"]
