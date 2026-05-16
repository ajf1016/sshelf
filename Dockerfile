FROM golang:1.23-alpine AS builder

ARG VERSION=dev
ARG TARGETOS=linux
ARG TARGETARCH=amd64

RUN apk add --no-cache git ca-certificates

WORKDIR /src

# Cache modules separately from source
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build \
      -trimpath \
      -ldflags="-s -w -X github.com/ajf1016/sshelf/internal/config.AppVersion=${VERSION}" \
      -o /out/sshelf \
      ./cmd/sshelf


FROM alpine:3.20

# sshelf shells out to ssh-keygen and ssh-agent at runtime
RUN apk add --no-cache \
      openssh-keygen \
      openssh-client \
      ca-certificates \
 && adduser -D -h /home/sshelf sshelf

USER sshelf
WORKDIR /home/sshelf

COPY --from=builder /out/sshelf /usr/local/bin/sshelf

ENTRYPOINT ["sshelf"]
CMD ["--help"]