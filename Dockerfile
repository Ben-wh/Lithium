# --- Frontend ---
FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package*.json ./
RUN npm install --no-audit --no-fund
COPY web/ ./
RUN npm run build

# --- Backend ---
FROM golang:1.26-alpine AS server
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/build ./web/build
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/lithium ./cmd/lithium \
 && mkdir -p /out/data

# --- Runtime ---
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=server /out/lithium /lithium
COPY --from=server --chown=nonroot:nonroot /out/data /data
ENV LITHIUM_DATA_DIR=/data \
    LITHIUM_ADDR=:8080
VOLUME /data
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s CMD ["/lithium", "healthcheck"]
ENTRYPOINT ["/lithium"]
