# Mahjong Fun: one image with the Go server and the built frontend.
#
#   docker build -t mahjong-fun .
#   docker run -p 8080:8080 mahjong-fun        # open http://localhost:8080
#
# The server listens on $PORT (default 8080), as most hosting platforms expect.

FROM node:22-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.22-alpine AS server
WORKDIR /src
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /mahjong ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=server /mahjong /mahjong
COPY --from=web /web/dist /web
ENV PORT=8080
EXPOSE 8080
ENTRYPOINT ["/mahjong", "-web", "/web"]
