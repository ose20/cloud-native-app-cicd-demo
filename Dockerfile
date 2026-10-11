ARG GO_VERSION
ARG APP_VERSION=dev
FROM golang:${GO_VERSION}-alpine3.24 AS build
ARG APP_VERSION
WORKDIR /build
COPY . .
RUN CGO_ENABLED=0 go build -ldflags "-s -w -X main.version=${APP_VERSION}" -trimpath -o app -a main.go

FROM gcr.io/distroless/static-debian13:nonroot
COPY --chown=nonroot:nonroot --from=build /build/app /app

USER nonroot

EXPOSE 8080

ENTRYPOINT [ "/app" ]
