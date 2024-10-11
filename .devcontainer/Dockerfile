FROM node:22-alpine

RUN apk add --no-cache git curl zsh unzip

ENV GO_VERSION=1.23.1
ENV PATH="/usr/local/go/bin:${PATH}"
RUN curl -fsSL https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz | tar -xz -C /usr/local && \
    rm -rf /usr/local/go${GO_VERSION}.linux-amd64.tar.gz