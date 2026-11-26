FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY main.go ./
COPY internal/ internal/
ARG VERSION=v0.4.1
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /floodline .

FROM scratch
COPY --from=build /floodline /floodline
USER 65532:65532
ENV PORT=8080 GOMAXPROCS=1 CAPTURE_SAMPLES=1
EXPOSE 8080
ENTRYPOINT ["/floodline"]
