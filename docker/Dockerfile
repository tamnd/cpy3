# Reproducible Linux amd64 test environment for cpy3.
#
# Builds CPython 3.14 from python.org twice: once with the GIL (standard)
# and once with --disable-gil (PEP 779 free-threaded, invoked as
# python3.14t), then installs Go 1.26 and runs `go test ./...` against
# both builds.
#
# Usage:
#   docker build -t cpy3-test -f docker/Dockerfile .
#   docker run --rm cpy3-test               # runs both test passes
#   docker run --rm cpy3-test gil           # run only the GIL pass
#   docker run --rm cpy3-test nogil         # run only the free-threaded pass
#   docker run --rm -it cpy3-test shell     # drop into a shell

ARG PYTHON_VERSION=3.14.4
ARG GO_VERSION=1.26.2

FROM ubuntu:24.04 AS builder

ARG PYTHON_VERSION
ARG GO_VERSION
ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends \
        build-essential \
        ca-certificates \
        curl \
        git \
        pkg-config \
        libssl-dev \
        libffi-dev \
        libbz2-dev \
        liblzma-dev \
        libreadline-dev \
        libsqlite3-dev \
        libncurses-dev \
        uuid-dev \
        zlib1g-dev \
        tk-dev \
        xz-utils \
    && rm -rf /var/lib/apt/lists/*

# Install Go 1.26.
RUN curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" \
        -o /tmp/go.tgz \
    && tar -C /usr/local -xzf /tmp/go.tgz \
    && rm /tmp/go.tgz
ENV PATH=/usr/local/go/bin:/root/go/bin:${PATH}
ENV GOPATH=/root/go

# Fetch CPython source once; each build uses its own objdir so we do not
# have to re-extract the tarball.
RUN curl -fsSL "https://www.python.org/ftp/python/${PYTHON_VERSION}/Python-${PYTHON_VERSION}.tgz" \
        -o /tmp/Python.tgz \
    && mkdir -p /usr/src \
    && tar -xzf /tmp/Python.tgz -C /usr/src \
    && rm /tmp/Python.tgz

# GIL build. Installs to /opt/python-gil; exposes python3.14 and
# python-3.14-embed.pc under that prefix.
WORKDIR /usr/src/Python-${PYTHON_VERSION}
RUN mkdir -p /tmp/build-gil \
    && cd /tmp/build-gil \
    && /usr/src/Python-${PYTHON_VERSION}/configure \
        --prefix=/opt/python-gil \
        --enable-shared \
        --with-ensurepip=no \
        --disable-test-modules \
    && make -j"$(nproc)" \
    && make install \
    && rm -rf /tmp/build-gil

# Free-threaded build. Installs to /opt/python-nogil; exposes python3.14t
# and python-3.14t-embed.pc. --disable-gil is a PEP 779 configure flag
# that ships a separate interpreter, so the two installs coexist without
# clobbering each other.
RUN mkdir -p /tmp/build-nogil \
    && cd /tmp/build-nogil \
    && /usr/src/Python-${PYTHON_VERSION}/configure \
        --prefix=/opt/python-nogil \
        --enable-shared \
        --disable-gil \
        --with-ensurepip=no \
        --disable-test-modules \
    && make -j"$(nproc)" \
    && make install \
    && rm -rf /tmp/build-nogil /usr/src/Python-${PYTHON_VERSION}

# The free-threaded install names its embed pkg-config file
# python-3.14t-embed.pc. The cpy3 cgo directive targets
# python-3.14-embed, so add a symlink so the same source tree builds
# against both installs without modification.
RUN ln -sf python-3.14t-embed.pc \
        /opt/python-nogil/lib/pkgconfig/python-3.14-embed.pc || true

# Runtime image keeps both Python installs, Go, and the repo. It is small
# enough (~1.5 GB) that pushing it to a registry is feasible, and large
# enough that nothing is missing for `go test`.
FROM ubuntu:24.04

ARG PYTHON_VERSION
ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends \
        build-essential \
        ca-certificates \
        pkg-config \
        libssl3 \
        libffi8 \
        libbz2-1.0 \
        liblzma5 \
        libreadline8 \
        libsqlite3-0 \
        libncurses6 \
        uuid-runtime \
        zlib1g \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /opt/python-gil   /opt/python-gil
COPY --from=builder /opt/python-nogil /opt/python-nogil
COPY --from=builder /usr/local/go     /usr/local/go

ENV PATH=/usr/local/go/bin:/root/go/bin:/opt/python-gil/bin:${PATH}
ENV GOPATH=/root/go
ENV LD_LIBRARY_PATH=/opt/python-gil/lib:/opt/python-nogil/lib

WORKDIR /src
COPY . /src

COPY docker/run-tests.sh /usr/local/bin/run-tests
RUN chmod +x /usr/local/bin/run-tests

ENTRYPOINT ["/usr/local/bin/run-tests"]
CMD ["all"]
