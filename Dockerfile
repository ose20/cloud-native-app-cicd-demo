ARG GO_VERSION
FROM golang:${GO_VERSION}-alpine3.24 AS build
WORKDIR /build
COPY . .
RUN CGO_ENABLED=0 go build -ldflags '-s -w' -trimpath -o app -a main.go

FROM gcr.io/distroless/static-debian13:nonroot
COPY --chown=nonroot:nonroot --from=build /build/app /app

USER nonroot

EXPOSE 8080

ENTRYPOINT [ "/app" ]
