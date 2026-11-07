# Stage 1: Build Frontend
FROM node:22-alpine AS ui-builder
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Stage 2: Build Go Binary
FROM golang:1.24-alpine AS go-builder
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
COPY --from=ui-builder /app/web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /app/faultcraft ./cmd/faultcraft

# Stage 3: Scratch / Minimal Production Image
FROM alpine:3.20
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /root/
COPY --from=go-builder /app/faultcraft /usr/local/bin/faultcraft

EXPOSE 8474
ENTRYPOINT ["faultcraft"]
CMD ["start", "--api-addr", ":8474"]
