# Build Step
FROM golang:1.23.4-alpine@sha256:c23339199a08b0e12032856908589a6d41a0dab141b8b3b21f156fc571a3f1d3

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

