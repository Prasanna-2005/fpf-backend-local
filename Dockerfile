# Python dependencies stage (manylinux wheels; Alpine/musl has no PyMuPDF wheels)
FROM python:3.11-slim-bookworm AS python-builder

COPY requirements.txt .

RUN pip install --no-cache-dir uv && uv pip install --system --no-cache-dir -r requirements.txt

# Go build stage
FROM golang:1.24-alpine AS go-builder

RUN apk add --no-cache git build-base

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o bin/formpf .

# Final runtime stage (glibc, same Python as python-builder)
FROM python:3.11-slim-bookworm AS runner

RUN apt-get update \
 && apt-get install --no-install-recommends -y tzdata \
 && rm -rf /var/lib/apt/lists/* \
 && useradd -r formpf \
 && mkdir -p /var/storage/formpf/

WORKDIR /app

COPY --from=python-builder /usr/local/lib/python3.11/site-packages /usr/local/lib/python3.11/site-packages

COPY --from=go-builder /app/bin /app/bin
COPY --from=go-builder /app/config ./config
COPY --from=go-builder /app/static ./static
COPY --from=go-builder /app/entrypoint.sh /app/entrypoint.sh
COPY --from=go-builder /app/.env ./.env

COPY modules/pymupdf_table_extraction.py ./modules/

RUN chmod +x /app/entrypoint.sh

EXPOSE 8080

ENTRYPOINT ["/app/entrypoint.sh"]
CMD ["./bin/formpf"]
