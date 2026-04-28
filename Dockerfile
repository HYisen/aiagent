# This Dockerfile achieve the goal of docs/Dockerfile in static link way,
# results in a even smaller binary...

# latest version
FROM golang:1.26.2-alpine3.23 AS build

RUN apk add gcc musl-dev git # git for vcs in debug.BuildInfo
WORKDIR /app
COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
  go mod download
RUN go generate ./...
RUN go test ./...
RUN env CGO_ENABLED=1 go build -ldflags '-extldflags "-static"'

FROM scratch
COPY --from=build /app/aiagent /
ENTRYPOINT ["/aiagent"]