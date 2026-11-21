# CommitGen in a container: no Go, no install, just Docker.
#
#   docker build -t commitgen .
#   docker run --rm -v "$PWD:/repo" commitgen -p --offline
#   docker run --rm -v "$PWD:/repo" -e GROQ_API_KEY commitgen -p
#   docker run --rm -v "$PWD:/repo" commitgen changelog

FROM golang:1-alpine AS build
WORKDIR /src
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/CommitGen .

FROM alpine:3
# safe.directory: the mounted repo is owned by your user, not the container's.
RUN apk add --no-cache git ca-certificates \
    && git config --system --add safe.directory '*' \
    && adduser -D -u 10001 commitgen
COPY --from=build /out/CommitGen /usr/local/bin/CommitGen
USER commitgen
WORKDIR /repo
ENTRYPOINT ["CommitGen"]
CMD ["--help"]
