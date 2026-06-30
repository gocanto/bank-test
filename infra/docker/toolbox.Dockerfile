ARG GO_VERSION
FROM golang:${GO_VERSION}

RUN apt-get update \
    && apt-get install -y --no-install-recommends bash ca-certificates curl git docker.io \
    && rm -rf /var/lib/apt/lists/*

RUN curl -L https://encore.dev/install.sh | bash
RUN curl -sSf https://temporal.download/cli.sh | sh

ENV PATH="/root/.encore/bin:/root/.temporalio/bin:${PATH}"

WORKDIR /workspace
