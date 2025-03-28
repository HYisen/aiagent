# The Goal of this Dockerfile is to help me build binary that can be used in my server, blame on CGO.
# Normally we could just switch gorm.io/driver/sqlite to github.com/glebarez/sqlite, maybe I just wanted
# to challenge my self. The situation is, the server uses GLIBC 2.24 on debian stretch and I use go latest.
# That version of OS is outdated and no longer maintained by official, which means `apt install` is gone.
FROM alpine:3.21.3 AS certs

# That's the gcc version running on my server.
# Considering it's the heaviest among all the dependencies,
# we don't make it ourselves, just use one as the base.
FROM gcc:6.3.0 AS build

# Support HTTPS, otherwise go get would fail with TLS errors.
# I also find that crt file in my Linux dev machine,
# just get one from another image is more elegant.
# Because that OS version's package registry has gone,
# we can not use `apt install ca-certificates` instead.
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# The install path is suggested by https://go.dev/doc/install.
# I choosed that version because it's the lateset and matchs server machine.
ADD "https://go.dev/dl/go1.24.1.linux-amd64.tar.gz" go.tar.gz
RUN tar -C /usr/local -xzf go.tar.gz
# However we don't modify $PATH, because the base image contains go1.6.1.
# And that's also why we use /usr/local/go/bin/go rather than go later.
# Shame on the blog with liar examples that ADD url could unarchive.
# https://www.docker.com/blog/docker-best-practices-understanding-the-differences-between-add-and-copy-instructions-in-dockerfiles/
# At present, ADD unarchive featuer is documented as only for local <src>.
# https://docs.docker.com/reference/dockerfile/#source
# https://github.com/moby/moby/issues/2369
# https://github.com/moby/buildkit/issues/4482
# Once the accpeted feature is done, one should reduce the usage of RUN to unarchive.

# Import all the source codes.
# As long as it is a clean build scratched from git clone, there shall be no executable.
# Some of the tools and documents are useless here, we tolerant becasue of their lightweight.
# Note things like docs/ddl.sql and tools/gen are mandatory to build.
COPY . /app

WORKDIR /app
RUN --mount=type=cache,target=/root/.cache/go-build \
  /usr/local/go/bin/go mod download
RUN /usr/local/go/bin/go run tools/gen/main.go # generate generated code
RUN /usr/local/go/bin/go build

FROM scratch
COPY --from=build /app/aiagent /
ENTRYPOINT ["/aiagent"]

# Last but not the least, I shall inform you that, introducing static link can also achieve the goal.
# Refer to Dockerfile under the project ROOT. (Hopefully you have read it first.)