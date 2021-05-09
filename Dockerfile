FROM golang AS builder

WORKDIR /
ADD . ./

RUN CGO_ENABLED=0 GOOS=linux go build -o main ./cmd

FROM ubuntu:18.04
RUN \
      apt-get update && \
      apt-get -qqy install --fix-missing \
            curl \
            vim \
            ca-certificates \
            iputils-ping \
      && \
      apt-get clean

EXPOSE 8080
COPY --from=builder ./main /main

ENTRYPOINT ["/main"]