#!/bin/bash
set -e

CERT_DIR="/var/lib/postgresql/certs"
SSL_CONF="/var/lib/postgresql/ssl.conf"

if [ "${SSL_ENABLED:-false}" = "true" ]; then
    echo "SSL enabled, configuring PostgreSQL to use certificates..."
    cat >"$SSL_CONF" <<EOF
ssl = on
ssl_cert_file = '$CERT_DIR/server.crt'
ssl_key_file = '$CERT_DIR/server.key'
EOF
    echo "SSL configuration written to $SSL_CONF"
else
    echo "SSL disabled"
    cat >"$SSL_CONF" <<EOF
ssl = off
EOF
fi

chown postgres:postgres "$SSL_CONF"
