FROM golang:1.25-bookworm AS build
RUN apt-get update && apt-get install -y --no-install-recommends libopus-dev libsoxr-dev pkg-config && rm -rf /var/lib/apt/lists/*
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=1 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/mumabridge ./cmd/mumabridge

FROM gcr.io/distroless/cc-debian12:nonroot
COPY --from=build /usr/lib/x86_64-linux-gnu/libopus.so.0 /usr/lib/x86_64-linux-gnu/libopus.so.0
COPY --from=build /usr/lib/x86_64-linux-gnu/libsoxr.so.0 /usr/lib/x86_64-linux-gnu/libsoxr.so.0
COPY --from=build /out/mumabridge /usr/local/bin/mumabridge
VOLUME ["/var/lib/mumabridge"]
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/mumabridge"]
