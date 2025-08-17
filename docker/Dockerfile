FROM golang:1.22 AS build

# build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o build -o /bin/orders ./cmd/orders

# run
FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=build /bin/orders /usr/local/bin/orders
COPY ./web /app/web
ENV HTTP_ADDR="8081"
ENTRYPOINT ["/usr/local/bin/orders"]