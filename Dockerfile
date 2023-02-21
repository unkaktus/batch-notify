FROM golang:alpine AS build
WORKDIR /code
COPY . .
RUN --mount=type=cache,target=/go/pkg --mount=type=cache,target=/root/.cache/go-build \
    go build -v ./cmd/batch-notify

FROM alpine
COPY --from=build /code/batch-notify /code/config.json /
ENTRYPOINT [ "/batch-notify" ]