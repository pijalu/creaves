# This is a multi-stage Dockerfile and requires >= Docker 17.05
# https://docs.docker.com/engine/userguide/eng-image/multistage-build/
FROM golang:alpine AS builder

ENV GOPROXY http://proxy.golang.org
RUN go install github.com/gobuffalo/cli/cmd/buffalo@latest
RUN apk add --no-cache npm yarn
RUN mkdir -p /src/creaves
WORKDIR /src/creaves

# this will cache the npm install step, unless package.json changes
ADD package.json .
RUN npm install
RUN yarn install
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

ADD . .
RUN buffalo plugins install
RUN buffalo build --environment development -o /bin/app

FROM alpine

ARG TZ='Europe/Brussels'
ENV DEFAULT_TZ ${TZ}

RUN apk add --no-cache bash ca-certificates tzdata netcat-openbsd \
  && cp /usr/share/zoneinfo/${DEFAULT_TZ} /etc/localtime

WORKDIR /bin/

COPY --from=builder /bin/app .

# set assets env for dev
ENV GO_ENV development

# Bind the app to 0.0.0.0 so it can be seen from outside the container
ENV ADDR 0.0.0.0

EXPOSE 3000
