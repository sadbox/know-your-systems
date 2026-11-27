# The one place the PDFs and the website are built: the same Chrome, fonts
# and font rendering on the Mac (linux/arm64) and in CI (linux/amd64), so
# the fit check gives the same answer everywhere. ./kys (and ./build, which
# uses it) builds and runs it, locally and in CI.

# BASE: the build container's base (base/Dockerfile: Debian, Chromium,
# poppler, CA certificates). ./kys passes the tag made from that file's
# contents (ghcr.io/sadbox/know-your-systems-base:chromium-<version>-<hash>),
# pulled from GHCR or, before base.yaml has published it, built from
# base/Dockerfile. The default is only for a plain `docker build`.
ARG BASE=ghcr.io/sadbox/know-your-systems-base:latest

# Go: builds the generator. go.yml runs the Go checks in the `test` target.
FROM golang:1.27.4@sha256:5f1b98fa02fe396ebce2a0227a0e08309e11792915b9dd2e1f71053d7e7e6666 AS build
WORKDIR /src
COPY go.mod ./
COPY *.go ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /kys .

FROM build AS test

# hadolint ignore=DL3006
FROM ${BASE}
COPY --from=build /kys /usr/local/bin/kys
# The repository is mounted at /work: templates, assets and EDITION are read
# from the checkout, and the PDFs and site are written back into it. The
# container is Chrome's sandbox (KYS_CONTAINER: run it with --no-sandbox),
# and Chrome needs a writable home whatever --user the container runs as.
ENV CHROME=/usr/bin/chromium KYS_CONTAINER=1 HOME=/tmp
WORKDIR /work
ENTRYPOINT ["/usr/local/bin/kys"]
