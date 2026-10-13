FROM golang:1.27.1-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/manafield-noctite ./cmd/manafield-noctite

FROM scratch
COPY --from=build /out/manafield-noctite /manafield-noctite
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/manafield-noctite"]
