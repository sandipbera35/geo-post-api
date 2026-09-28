# --- build stage ---
FROM golang:1.27-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/geo-post-api .

# --- runtime stage ---
FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app

COPY --from=build /out/geo-post-api ./geo-post-api
COPY --from=build /src/cert ./cert

# config.Load()'s default JWT_PUBLIC_KEY_PATH is computed at compile time
# from the build stage's source path (/src/...), which doesn't exist in
# this runtime image — set it explicitly so it matches where cert/ actually
# landed here. Still overridable via `docker run -e JWT_PUBLIC_KEY_PATH=...`.
ENV JWT_PUBLIC_KEY_PATH=/app/cert/public_key.pem

EXPOSE 8080
ENTRYPOINT ["./geo-post-api"]
