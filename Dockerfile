# Build Step
FROM golang:1.23.5-alpine@sha256:47d337594bd9e667d35514b241569f95fb6d95727c24b19468813d596d5ae596

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

