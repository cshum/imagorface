ARG GOLANG_VERSION=1.26.3
ARG BASE_IMAGE=ghcr.io/cshum/imagor-base:vips8.18.2-r9
ARG DEV_BASE_IMAGE=${BASE_IMAGE}-dev

FROM golang:${GOLANG_VERSION}-bookworm AS golang-base

FROM ${BASE_IMAGE} AS native-base

FROM ${DEV_BASE_IMAGE} AS builder

ARG ENABLE_MAGICK=false
ARG ENABLE_MOZJPEG=false

COPY --from=golang-base /usr/local/go /usr/local/go

ENV GOPATH=/go
ENV PATH=/usr/local/go/bin:/go/bin:$PATH
ENV CGO_ENABLED=1
ENV PKG_CONFIG_PATH=/opt/imagor/lib/pkgconfig
ENV CGO_CFLAGS=-I/opt/imagor/include
ENV CGO_LDFLAGS="-L/opt/imagor/lib -Wl,-rpath,/opt/imagor/lib"

WORKDIR ${GOPATH}/src/github.com/cshum/imagorface

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN go build -ldflags "-s -w" -o ${GOPATH}/bin/imagorface ./cmd/imagorface/main.go

FROM native-base AS runtime
LABEL maintainer="adrian@cshum.com"

ARG ENABLE_MAGICK=false
ARG ENABLE_MOZJPEG=false

RUN DEBIAN_FRONTEND=noninteractive \
  apt-get update && \
  apt-get upgrade -y && \
  apt-get install --no-install-recommends -y \
  media-types && \
  if [ "$ENABLE_MAGICK" = "true" ]; then \
    apt-get install --no-install-recommends -y imagemagick; \
  fi && \
  ln -s /usr/lib/$(uname -m)-linux-gnu/libjemalloc.so.2 /usr/local/lib/libjemalloc.so && \
  mkdir -p /var/cache/fontconfig && \
  chmod 777 /var/cache/fontconfig && \
  rm -rf /var/lib/apt/lists/* && \
  rm -rf /etc/fonts/conf.d/10-sub-pixel-rgb.conf /etc/fonts/conf.d/11-lcdfilter-default.conf

COPY --from=builder /go/bin/imagorface /opt/imagor/bin/imagorface
RUN ln -s /opt/imagor/bin/imagorface /usr/local/bin/imagorface

ENV VIPS_WARNING=0
ENV MALLOC_ARENA_MAX=2
ENV LD_PRELOAD=/usr/local/lib/libjemalloc.so
ENV LD_LIBRARY_PATH=/opt/imagor/lib
ENV FONTCONFIG_PATH=/etc/fonts
ENV XDG_CACHE_HOME=/tmp

ENV PORT=8000

# use unprivileged user
USER nobody

ENTRYPOINT ["/usr/local/bin/imagorface"]

EXPOSE ${PORT}
