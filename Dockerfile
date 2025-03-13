# Build Step
FROM golang:1.24.1-alpine@sha256:43c094ad24b6ac0546c62193baeb3e6e49ce14d3250845d166c77c25f64b0386

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

