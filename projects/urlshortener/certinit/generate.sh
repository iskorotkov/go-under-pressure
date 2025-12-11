#!/bin/sh
set -e

apk add --no-cache openssl

# PostgreSQL cert (ECDSA P-256)
if [ ! -f /certs/postgres/server.crt ]; then
    echo "Generating PostgreSQL certificate..."
    openssl ecparam -genkey -name prime256v1 -noout -out /certs/postgres/server.key
    openssl req -new -x509 -sha256 -days 3650 -nodes \
        -key /certs/postgres/server.key \
        -subj "/CN=localhost" \
        -addext "subjectAltName=DNS:localhost,DNS:postgres,IP:127.0.0.1" \
        -out /certs/postgres/server.crt
    chmod 600 /certs/postgres/server.key
    chmod 644 /certs/postgres/server.crt
    chown 70:70 /certs/postgres/server.key /certs/postgres/server.crt
fi

# API cert (ECDSA P-256)
if [ ! -f /certs/api/server.crt ]; then
    echo "Generating API certificate for $API_HOST..."
    openssl ecparam -genkey -name prime256v1 -noout -out /certs/api/server.key
    openssl req -new -x509 -sha256 -days 3650 -nodes \
        -key /certs/api/server.key \
        -subj "/CN=$API_HOST" \
        -addext "subjectAltName=DNS:$API_HOST,DNS:localhost,DNS:api,IP:127.0.0.1" \
        -out /certs/api/server.crt
    chmod 644 /certs/api/server.key /certs/api/server.crt
fi

# Grafana cert (ECDSA P-256)
if [ ! -f /certs/grafana/server.crt ]; then
    echo "Generating Grafana certificate for $GRAFANA_HOST..."
    openssl ecparam -genkey -name prime256v1 -noout -out /certs/grafana/server.key
    openssl req -new -x509 -sha256 -days 3650 -nodes \
        -key /certs/grafana/server.key \
        -subj "/CN=$GRAFANA_HOST" \
        -addext "subjectAltName=DNS:$GRAFANA_HOST,DNS:localhost,DNS:grafana,IP:127.0.0.1" \
        -out /certs/grafana/server.crt
    chmod 644 /certs/grafana/server.key /certs/grafana/server.crt
fi

echo "Certificates generated successfully"
