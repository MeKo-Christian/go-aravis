FROM golang:1.23-bookworm

WORKDIR /src

# Aravis 0.8 development headers/libraries plus the CGO build toolchain.
# Debian bookworm ships aravis 0.8.x as the `libaravis-dev` package, which
# provides the aravis-0.8 pkg-config module this binding links against.
RUN apt-get update && apt-get install -y --no-install-recommends \
        build-essential \
        pkg-config \
        libaravis-dev \
    && rm -rf /var/lib/apt/lists/*

COPY . /src

RUN CGO_ENABLED=1 go build -o /usr/local/bin/listdevices ./examples/list_devices

CMD ["listdevices"]
