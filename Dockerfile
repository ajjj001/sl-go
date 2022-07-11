FROM golang:1.17.2-alpine3.14 AS builder

RUN mkdir /app
WORKDIR /app

RUN apk add git libc-dev build-base
COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./
RUN go build -o docker-app .

FROM alpine:3.14
RUN apk update \
    && apk add ca-certificates \
    && rm -rf /var/cache/apk/*
WORKDIR /app

COPY --from=builder app/docker-app .

EXPOSE 8080
CMD [ "./docker-app" ]

#docker build --tag docker-gs-ping .
#docker run -d -p 4000:4000 -e PORT=4000 docker-gs-ping