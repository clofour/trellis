FROM golang:1.24-alpine AS build
ARG TUTORIAL_VERSION=dev
WORKDIR /src
COPY go.mod main.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w -X main.version=${TUTORIAL_VERSION}" -o /out/trellis-tutorial .

FROM scratch
COPY --from=build /out/trellis-tutorial /trellis-tutorial
EXPOSE 8080
ENTRYPOINT ["/trellis-tutorial"]
