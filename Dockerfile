FROM golang:1.25-alpine AS build

WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o /bin/notifier-api ./cmd/notifier-api

FROM alpine:3.21

WORKDIR /app
COPY --from=build /bin/notifier-api /usr/local/bin/notifier-api
EXPOSE 8080
ENTRYPOINT ["notifier-api"]
