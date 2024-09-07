FROM golang:1.22 AS builder

RUN mkdir /src
WORKDIR /src


COPY go.mod go.sum .
RUN --mount=type=cache,target=/go/pkg/mod \
	go mod download -x

COPY . .

ARG VERSION

RUN \
	--mount=type=cache,target=/go/pkg/mod \
	--mount=type=cache,target=/root/.cache/go-build \
	CGO_ENABLED=0 go build -ldflags="-X main.Version=$VERSION" -v -o /j5 \
	./cmd/j5

FROM scratch

LABEL org.opencontainers.image.source=https://github.com/pentops/j5

COPY --from=builder /j5 /j5
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

WORKDIR /src
CMD []
ENTRYPOINT ["/j5"]
