# syntax=docker/dockerfile:1.7

FROM --platform=$BUILDPLATFORM golang:1.25-bookworm AS builder

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG COMMIT_SHA=unknown
ARG BUILD_DATE=unknown

WORKDIR /src

ENV CGO_ENABLED=0 \
    GOFLAGS=-trimpath

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build \
      -ldflags="-s -w -X vesko/pkg/buildinfo.Version=${VERSION} -X vesko/pkg/buildinfo.Commit=${COMMIT_SHA} -X vesko/pkg/buildinfo.BuildDate=${BUILD_DATE}" \
      -o /out/wesko-api \
      .

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=builder /out/wesko-api /app/wesko-api

EXPOSE 8080

ENTRYPOINT ["/app/wesko-api"]
