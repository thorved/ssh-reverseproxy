FROM node:20-alpine AS frontend-builder

WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci

COPY frontend/ ./
RUN npm run build

FROM golang:1.25-alpine AS backend-builder

WORKDIR /app/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./
RUN GOOS=linux GOARCH=amd64 go build -o server ./cmd/server

FROM alpine:3.22

WORKDIR /app

RUN apk add --no-cache ca-certificates

COPY --from=backend-builder /app/backend/server ./server
COPY --from=frontend-builder /app/frontend/out ./frontend/out

RUN mkdir -p /app/data

EXPOSE 8080 2222

CMD ["./server"]
