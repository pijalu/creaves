# This is a multi-stage Dockerfile and requires >= Docker 17.05
# https://docs.docker.com/engine/userguide/eng-image/multistage-build/
FROM gobuffalo/buffalo:v0.16.23-slim as builder

ENV GO111MODULE on
ENV GOPROXY http://proxy.golang.org

# Upgrade buffalo with sqlite3 support
RUN go get -u -v -tags sqlite github.com/gobuffalo/buffalo/buffalo
RUN go get -u -v -tags sqlite github.com/gobuffalo/buffalo-pop/v2

RUN mkdir -p /src/creaves
WORKDIR /src/creaves

# this will cache the npm install step, unless package.json changes
ADD package.json .
ADD yarn.lock .
RUN yarn install --no-progress
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download
ADD . .
RUN buffalo build --environment production --static -o /bin/app

FROM alpine
ARG TZ='Europe/Brussels'
ENV DEFAULT_TZ ${TZ}

RUN apk add --no-cache bash ca-certificates tzdata \
  && cp /usr/share/zoneinfo/${DEFAULT_TZ} /etc/localtime 
  
WORKDIR /bin/
COPY --from=builder /bin/app .
COPY dockerscript/* /bin/
# Uncomment to run the binary in "production" mode:
ENV GO_ENV=production
# Bind the app to 0.0.0.0 so it can be seen from outside the container
ENV ADDR=0.0.0.0

EXPOSE 3000

CMD /bin/quickstart.prod.sh
