FROM golang:1.14-alpine

ARG user=scraper
ARG group=scraper
ARG uid=1010
ARG gid=1010

WORKDIR /src

RUN apk add --no-cache curl

COPY /src /src
RUN go build /src .

RUN mkdir /app \
  && mv /src/scraper /app/scraper \
  && rm -rf /src \
  && chmod +x /app/scraper \
  && addgroup -g ${gid} ${group} \
  && adduser -u ${uid} -G ${group} -s /bin/bash -D ${user} \
  && chown ${uid}:${gid} /app/scraper

USER ${user}

HEALTHCHECK CMD curl --fail http://localhost:2112/metrics

ENTRYPOINT ["/app/scraper"]