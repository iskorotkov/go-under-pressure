#!/bin/sh
set -e

apk add --no-cache openssl

# PostgreSQL cert
if [ ! -f /certs/postgres/server.crt ]; then
    echo "Generating PostgreSQL certificate..."
    openssl req -new -x509 -days 3650 -nodes \
        -subj "/CN=localhost" \
        -addext "subjectAltName=DNS:localhost,DNS:postgres,IP:127.0.0.1" \
        -keyout /certs/postgres/server.key \
        -out /certs/postgres/server.crt
    chmod 600 /certs/postgres/server.key
    chmod 644 /certs/postgres/server.crt
    chown 70:70 /certs/postgres/server.key /certs/postgres/server.crt
fi

# API cert
if [ ! -f /certs/api/server.crt ]; then
    echo "Generating API certificate for $API_HOST..."
    openssl req -new -x509 -days 3650 -nodes \
        -subj "/CN=$API_HOST" \
        -addext "subjectAltName=DNS:$API_HOST,DNS:localhost,DNS:api,IP:127.0.0.1" \
        -keyout /certs/api/server.key \
        -out /certs/api/server.crt
    chmod 644 /certs/api/server.key /certs/api/server.crt
fi

# Grafana cert
if [ ! -f /certs/grafana/server.crt ]; then
    echo "Generating Grafana certificate for $GRAFANA_HOST..."
    openssl req -new -x509 -days 3650 -nodes \
        -subj "/CN=$GRAFANA_HOST" \
        -addext "subjectAltName=DNS:$GRAFANA_HOST,DNS:localhost,DNS:grafana,IP:127.0.0.1" \
        -keyout /certs/grafana/server.key \
        -out /certs/grafana/server.crt
    chmod 644 /certs/grafana/server.key /certs/grafana/server.crt
fi

echo "Certificates generated successfully"
