# Build Step
FROM golang:1.23.4-alpine@sha256:04ec5618ca64098b8325e064aa1de2d3efbbd022a3ac5554d49d5ece99d41ad5

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

