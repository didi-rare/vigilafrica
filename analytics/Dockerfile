# Umami, re-based onto a patched Node.js runtime.
#
# Upstream only rebuilds its image when Umami itself releases, so a Node.js
# security release can leave the newest image on a vulnerable runtime for
# weeks: Umami 3.4.0 ships Node 22.23.2, while the 2026-10-21 Node.js security
# release (CVE-2026-111497 and others) is fixed in 22.23.4. This copies the
# patched node binary from the official image over the upstream one. Both
# images are Alpine 3.24 (musl), so the binary's libc and libstdc++ match.
#
# Delete this file and pin the upstream image directly once Umami publishes an
# image whose NODE_VERSION is at or above the patched release.
ARG UMAMI_IMAGE=ghcr.io/umami-software/umami:3.4.0@sha256:85909afc45bdcda1917394594a087421fdbb05610fded0fa9f6fb861abb2f367
ARG NODE_IMAGE=node:22.23.4-alpine3.24@sha256:fe4f6d74e20370acb04ad889077d14d0f64a9c8ea6a42bc60cddf1e844699914

FROM ${NODE_IMAGE} AS node

FROM ${UMAMI_IMAGE}
COPY --from=node /usr/local/bin/node /usr/local/bin/node
ENV NODE_VERSION=22.23.4
