# The build container's base: Debian with a pinned Chromium (the PDFs), poppler (the visual diff)
# and CA certificates (the link check), published as ghcr.io/sadbox/know-your-systems-base by
# .github/workflows/base.yaml. The generator's Dockerfile starts from it by digest, so everyday
# builds, deploys and checks pull it from GHCR and never touch snapshot.debian.org; only a change
# here does.
# Chromium from Debian, pinned three ways so a rebuild gets the same browser:
# the base image by digest, the package lists by a snapshot.debian.org date
# (nothing comes from the live archive),
# and the package by version. To update: move SNAPSHOT to a later date and
# CHROMIUM, POPPLER and CA_CERTIFICATES to the versions on it (Dependabot moves the image
# digest).
# (chromedp/headless-shell was tried first: its headless shell never
# finishes --dump-dom, on amd64 or arm64.)
FROM debian:trixie-slim@sha256:8aa5a7bf5a1955bddbe7d74fcdd0b91ab30e74362a8b73af672b0481b35b44a0
ARG SNAPSHOT=20261126T000000Z
ARG CHROMIUM=156.0.8078.108-1~deb13u1
# pdftoppm, for the visual diff (-diff).
ARG POPPLER=25.03.0-5+deb13u4
# CA certificates, for the link check (-links).
ARG CA_CERTIFICATES=20250419+deb13u1
# Only the snapshot's lists, over http: apt checks every list and package
# against Debian's signing keys, and the slim image has no CA certificates
# for https until this step installs them. A snapshot's Release files expire (Valid-Until); its content
# doesn't, so that check is off.
RUN printf 'Types: deb\nURIs: http://snapshot.debian.org/archive/debian/%s\nSuites: trixie trixie-updates\nComponents: main\nSigned-By: /usr/share/keyrings/debian-archive-keyring.gpg\n\nTypes: deb\nURIs: http://snapshot.debian.org/archive/debian-security/%s\nSuites: trixie-security\nComponents: main\nSigned-By: /usr/share/keyrings/debian-archive-keyring.gpg\n' "$SNAPSHOT" "$SNAPSHOT" > /etc/apt/sources.list.d/debian.sources \
 && apt-get -o Acquire::Check-Valid-Until=false update \
 && apt-get install --yes --no-install-recommends "chromium=$CHROMIUM" "poppler-utils=$POPPLER" "ca-certificates=$CA_CERTIFICATES" \
 && rm -rf /var/lib/apt/lists/*
LABEL org.opencontainers.image.source="https://github.com/sadbox/know-your-systems" \
      org.opencontainers.image.description="Debian with a pinned Chromium, poppler and CA certificates: the Know Your Systems build container's base"
