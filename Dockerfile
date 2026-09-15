ARG NODE_IMAGE=node:24-alpine
ARG GO_IMAGE=golang:1.26.8-alpine
FROM ${NODE_IMAGE} AS frontend
WORKDIR /src
COPY package.json package-lock.json .npmrc ./
RUN npm ci --no-audit --no-fund --registry=https://registry.npmjs.org
COPY index.html vite.config.ts tsconfig.json postcss.config.mjs ./
COPY components ./components
COPY hooks ./hooks
COPY lib ./lib
COPY web ./web
COPY public ./public
RUN npm run build

FROM ${GO_IMAGE} AS backend
WORKDIR /src
ENV CGO_ENABLED=0
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN go build -trimpath -ldflags='-s -w' -o /campus ./cmd/server

FROM scratch
COPY --from=backend /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=backend /campus /campus
COPY --from=frontend /src/dist /app/dist
WORKDIR /app
ENV HTTP_ADDR=:8080 STATIC_DIR=/app/dist
USER 65532:65532
EXPOSE 8080
HEALTHCHECK --interval=20s --timeout=3s --start-period=20s --retries=3 CMD ["/campus", "healthcheck"]
ENTRYPOINT ["/campus"]
