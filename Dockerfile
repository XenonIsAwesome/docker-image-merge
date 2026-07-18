FROM golang:1.24-alpine AS builder

ENV GOTOOLCHAIN=auto
RUN apk add --no-cache git

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /docker-imagemerge .

FROM scratch
COPY --from=builder /docker-imagemerge /docker-imagemerge
ENTRYPOINT ["/docker-imagemerge"]
