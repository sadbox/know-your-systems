# The one place the PDFs and the website are built: the same Chrome, fonts
# and font rendering on the Mac (linux/arm64) and in CI (linux/amd64), so
# the fit check gives the same answer everywhere. ./build runs it locally;
# ci.yaml and pages.yaml run it in CI. Both images are pinned by digest and
# updated by Dependabot (docker).

# Go: builds the generator. go.yml runs the Go checks in the `test` target.
FROM golang:1.27.4@sha256:5f1b98fa02fe396ebce2a0227a0e08309e11792915b9dd2e1f71053d7e7e6666 AS build
WORKDIR /src
COPY go.mod ./
COPY *.go ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /kys .

FROM build AS test

# Chromium from Debian, pinned three ways so a rebuild gets the same browser:
# the base image by digest, the package lists by a snapshot.debian.org date
# (nothing comes from the live archive),
# and the package by version. To update: move SNAPSHOT to a later date and
# CHROMIUM to the version on it (Dependabot moves the image digest).
# (chromedp/headless-shell was tried first: its headless shell never
# finishes --dump-dom, on amd64 or arm64.)
FROM debian:trixie-slim@sha256:8aa5a7bf5a1955bddbe7d74fcdd0b91ab30e74362a8b73af672b0481b35b44a0
ARG SNAPSHOT=20261126T000000Z
ARG CHROMIUM=156.0.8078.108-1~deb13u1
# Only the snapshot's lists, over http: apt checks every list and package
# against Debian's signing keys, and the slim image has no CA certificates
# for https. A snapshot's Release files expire (Valid-Until); its content
# doesn't, so that check is off.
RUN printf 'Types: deb\nURIs: http://snapshot.debian.org/archive/debian/%s\nSuites: trixie trixie-updates\nComponents: main\nSigned-By: /usr/share/keyrings/debian-archive-keyring.gpg\n\nTypes: deb\nURIs: http://snapshot.debian.org/archive/debian-security/%s\nSuites: trixie-security\nComponents: main\nSigned-By: /usr/share/keyrings/debian-archive-keyring.gpg\n' "$SNAPSHOT" "$SNAPSHOT" > /etc/apt/sources.list.d/debian.sources \
 && apt-get -o Acquire::Check-Valid-Until=false update \
 && apt-get install --yes --no-install-recommends "chromium=$CHROMIUM" \
 && rm -rf /var/lib/apt/lists/*
COPY --from=build /kys /usr/local/bin/kys
# The repository is mounted at /work: templates, assets and EDITION are read
# from the checkout, and the PDFs and site are written back into it. The
# container is Chrome's sandbox (KYS_CONTAINER: run it with --no-sandbox),
# and Chrome needs a writable home whatever --user the container runs as.
ENV CHROME=/usr/bin/chromium KYS_CONTAINER=1 HOME=/tmp
WORKDIR /work
ENTRYPOINT ["/usr/local/bin/kys"]
