FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -ldflags="-s -w" -o /claude-perms .

FROM alpine:3.21

COPY --from=builder /claude-perms /usr/local/bin/claude-perms

ENTRYPOINT ["claude-perms"]
