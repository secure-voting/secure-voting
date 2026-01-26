#!/usr/bin/env bash
set -euo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUT="$DIR/out"
mkdir -p "$OUT"

CA_KEY="$OUT/ca.key"
CA_PEM="$OUT/ca.pem"
SRV_KEY="$OUT/compute.key"
SRV_CSR="$OUT/compute.csr"
SRV_PEM="$OUT/compute.pem"
SRV_EXT="$OUT/compute.ext"

openssl genrsa -out "$CA_KEY" 4096
openssl req -x509 -new -nodes -key "$CA_KEY" -sha256 -days 3650 \
  -subj "/C=NL/ST=Noord-Holland/L=Amsterdam/O=secure-voting/OU=dev/CN=secure-voting-dev-ca" \
  -out "$CA_PEM"

openssl genrsa -out "$SRV_KEY" 4096
openssl req -new -key "$SRV_KEY" \
  -subj "/C=NL/ST=Noord-Holland/L=Amsterdam/O=secure-voting/OU=compute/CN=rust-compute" \
  -out "$SRV_CSR"

cat > "$SRV_EXT" <<'EOF'
basicConstraints=CA:FALSE
keyUsage = digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = rust-compute
DNS.2 = compute
DNS.3 = localhost
EOF

openssl x509 -req -in "$SRV_CSR" -CA "$CA_PEM" -CAkey "$CA_KEY" -CAcreateserial \
  -out "$SRV_PEM" -days 3650 -sha256 -extfile "$SRV_EXT"

echo "OK:"
echo "  CA:     $CA_PEM"
echo "  SERVER: $SRV_PEM"
echo "  KEY:    $SRV_KEY"
