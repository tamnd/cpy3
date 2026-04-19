#!/usr/bin/env bash
# Entrypoint for the cpy3 Docker test image.
#
# Runs `go test ./...` against the GIL build, the free-threaded build,
# or both, depending on the first argument.

set -euo pipefail

PY_VERSION="${PY_VERSION:-3.14}"

run_gil() {
    echo "==> Python ${PY_VERSION} GIL build"
    PKG_CONFIG_PATH=/opt/python-gil/lib/pkgconfig \
        go test -count=1 -cover ./...
}

run_nogil() {
    echo "==> Python ${PY_VERSION} free-threaded build (python3.14t)"
    # The free-threaded install's pkg-config dir ships a symlink from
    # python-3.14-embed.pc to python-3.14t-embed.pc (created in the
    # Dockerfile), so the same cgo directive resolves against the
    # free-threaded libpython.
    LD_LIBRARY_PATH=/opt/python-nogil/lib:${LD_LIBRARY_PATH:-} \
    PKG_CONFIG_PATH=/opt/python-nogil/lib/pkgconfig \
        go test -count=1 -cover ./...
}

case "${1:-all}" in
    gil)
        run_gil
        ;;
    nogil|free-threaded|ft)
        run_nogil
        ;;
    all)
        run_gil
        echo
        run_nogil
        ;;
    shell|bash)
        exec /bin/bash
        ;;
    *)
        echo "usage: run-tests [gil|nogil|all|shell]" >&2
        exit 2
        ;;
esac
