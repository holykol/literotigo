FROM golang:1.16-alpine as builder

RUN apk add --no-cache git

# If enabled scratch container crashes with weird error
ENV CGO_ENABLED=0
ENV GOPROXY=direct

WORKDIR /build
COPY go.mod go.sum ./
RUN go get -v ./...

COPY main.go *.html ./
RUN go build

FROM scratch
USER 1000

COPY --from=builder /build/literotigo /srv/literotigo

EXPOSE 8080
HEALTHCHECK CMD curl -f http://localhost:8080 || exit 1

ENTRYPOINT ["/srv/literotigo", "/var/database.jsonl"]