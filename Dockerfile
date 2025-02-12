# Build Step
FROM golang:1.24.0-alpine@sha256:5429efb7de864db15bd99b91b67608d52f97945837c7f6f7d1b779f9bfe46281

# Dependencies
RUN apk update && apk add --no-cache upx make git alsa-lib-dev gcc libc-dev

# Source
WORKDIR $GOPATH/src/github.com/Depado/parakeet
COPY go.mod go.sum ./
RUN go mod download
RUN go mod verify
COPY . .

# Build
RUN make packed

ENTRYPOINT ["./parakeet"]

