# Python dependencies stage (builds once, cached forever)
FROM alpine:3.18 AS python-builder

RUN apk add --no-cache \
    python3 \
    py3-pip \
    gcc \
    g++ \
    musl-dev \
    python3-dev \
    jpeg-dev \
    zlib-dev \
    freetype-dev \
    make

COPY requirements.txt .
RUN pip3 install --no-cache-dir --break-system-packages -r requirements.txt

# Go build stage
FROM golang:1.24-alpine AS go-builder

RUN apk add --no-cache git build-base

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o bin/formpf .

# Final runtime stage
FROM alpine:3.18 AS runner

RUN apk add --no-cache \
    tzdata \
    python3

RUN adduser -D -g '' formpf \
 && mkdir -p /var/storage/formpf/

WORKDIR /app

# Copy Python packages from python-builder (CACHED after first build!)
COPY --from=python-builder /usr/lib/python3.11/site-packages /usr/lib/python3.11/site-packages

# Copy Go binary and app files
COPY --from=go-builder /app/bin /app/bin
COPY --from=go-builder /app/config ./config
COPY --from=go-builder /app/static ./static
COPY --from=go-builder /app/entrypoint.sh /app/entrypoint.sh
COPY --from=go-builder /app/.env ./.env

# Copy Python script
COPY modules/pymupdf_table_extraction.py ./modules/

RUN chmod +x /app/entrypoint.sh

EXPOSE 8080

ENTRYPOINT ["/app/entrypoint.sh"]
CMD ["./bin/formpf"]