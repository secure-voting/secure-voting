#!/usr/bin/env bash
set -euo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUT="$DIR/out"
mkdir -p "$OUT"

rm -f "$OUT"/*.csr "$OUT"/*.ext "$OUT"/*.srl "$OUT"/mongo.server.pem

CA_KEY="$OUT/ca.key"
CA_PEM="$OUT/ca.pem"

gen_ca() {
  openssl genrsa -out "$CA_KEY" 4096
  openssl req -x509 -new -nodes -key "$CA_KEY" -sha256 -days 3650 \
    -subj "/C=NL/ST=Noord-Holland/L=Amsterdam/O=secure-voting/OU=dev/CN=secure-voting-dev-ca" \
    -addext "basicConstraints=critical,CA:TRUE" \
    -addext "keyUsage=critical,keyCertSign,cRLSign" \
    -addext "subjectKeyIdentifier=hash" \
    -out "$CA_PEM"
}

write_ext() {
  local ext_file="$1"
  shift

  {
    echo "basicConstraints=CA:FALSE"
    echo "keyUsage = digitalSignature, keyEncipherment"
    echo "extendedKeyUsage = serverAuth, clientAuth"
    echo "subjectAltName = @alt_names"
    echo
    echo "[alt_names]"
  } > "$ext_file"

  local dns_i=1
  local ip_i=1

  for san in "$@"; do
    case "$san" in
      DNS:*)
        echo "DNS.${dns_i} = ${san#DNS:}" >> "$ext_file"
        dns_i=$((dns_i + 1))
        ;;
      IP:*)
        echo "IP.${ip_i} = ${san#IP:}" >> "$ext_file"
        ip_i=$((ip_i + 1))
        ;;
      *)
        echo "unsupported SAN entry: $san" >&2
        exit 1
        ;;
    esac
  done
}

gen_server_cert() {
  local name="$1"
  local subject="$2"
  shift 2

  local key="$OUT/${name}.key"
  local csr="$OUT/${name}.csr"
  local pem="$OUT/${name}.pem"
  local ext="$OUT/${name}.ext"

  openssl genrsa -out "$key" 4096
  openssl req -new -key "$key" -subj "$subject" -out "$csr"
  write_ext "$ext" "$@"

  openssl x509 -req -in "$csr" -CA "$CA_PEM" -CAkey "$CA_KEY" -CAcreateserial \
    -out "$pem" -days 3650 -sha256 -extfile "$ext"
}

gen_kafka_truststore() {
  rm -f "$OUT/kafka.truststore.p12"

  if command -v keytool >/dev/null 2>&1; then
    keytool -importcert \
      -noprompt \
      -alias secure-voting-dev-ca \
      -file "$CA_PEM" \
      -keystore "$OUT/kafka.truststore.p12" \
      -storetype PKCS12 \
      -storepass changeit

    chmod 644 "$OUT/kafka.truststore.p12"
    return
  fi

  if command -v docker >/dev/null 2>&1; then
    docker run --rm \
      --user "$(id -u):$(id -g)" \
      -v "$OUT:/work" \
      eclipse-temurin:21-jre \
      keytool -importcert \
        -noprompt \
        -alias secure-voting-dev-ca \
        -file /work/ca.pem \
        -keystore /work/kafka.truststore.p12 \
        -storetype PKCS12 \
        -storepass changeit

    chmod 644 "$OUT/kafka.truststore.p12"
    return
  fi

  echo "keytool is required to generate Kafka truststore" >&2
  echo "Install OpenJDK or make Docker available for eclipse-temurin:21-jre" >&2
  exit 1
}

FRONTEND_TLS_HOSTS="${FRONTEND_TLS_HOSTS:-localhost,127.0.0.1,ts-frontend}"

gen_ca

gen_server_cert \
  compute \
  "/C=NL/ST=Noord-Holland/L=Amsterdam/O=secure-voting/OU=compute/CN=rust-compute" \
  DNS:rust-compute DNS:compute DNS:localhost

frontend_sans=()
IFS=',' read -ra HOSTS <<< "$FRONTEND_TLS_HOSTS"
for raw in "${HOSTS[@]}"; do
  host="$(echo "$raw" | xargs)"
  [[ -n "$host" ]] || continue
  if [[ "$host" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]]; then
    frontend_sans+=("IP:$host")
  else
    frontend_sans+=("DNS:$host")
  fi
done

gen_server_cert \
  frontend \
  "/C=NL/ST=Noord-Holland/L=Amsterdam/O=secure-voting/OU=frontend/CN=localhost" \
  "${frontend_sans[@]}"

gen_server_cert \
  db \
  "/C=NL/ST=Noord-Holland/L=Amsterdam/O=secure-voting/OU=postgres/CN=db" \
  DNS:db DNS:postgres-db DNS:localhost IP:127.0.0.1

gen_server_cert \
  redis \
  "/C=NL/ST=Noord-Holland/L=Amsterdam/O=secure-voting/OU=redis/CN=cache" \
  DNS:cache DNS:redis-cache DNS:localhost IP:127.0.0.1

gen_server_cert \
  mongo \
  "/C=NL/ST=Noord-Holland/L=Amsterdam/O=secure-voting/OU=mongo/CN=mongo" \
  DNS:mongo \
  DNS:mongo-db \
  DNS:mongo-secondary \
  DNS:mongo-db-secondary \
  DNS:localhost \
  IP:127.0.0.1

gen_server_cert \
  kafka \
  "/C=NL/ST=Noord-Holland/L=Amsterdam/O=secure-voting/OU=kafka/CN=kafka" \
  DNS:kafka DNS:localhost IP:127.0.0.1

openssl pkcs12 -export \
  -in "$OUT/kafka.pem" \
  -inkey "$OUT/kafka.key" \
  -certfile "$CA_PEM" \
  -out "$OUT/kafka.keystore.p12" \
  -name kafka \
  -passout pass:changeit

gen_kafka_truststore

echo "changeit" > "$OUT/kafka_keystore_creds"
echo "changeit" > "$OUT/kafka_sslkey_creds"
echo "changeit" > "$OUT/kafka_truststore_creds"

chmod 644 "$OUT/kafka.keystore.p12"
chmod 644 "$OUT/kafka.truststore.p12"
chmod 644 "$OUT/kafka_keystore_creds"
chmod 644 "$OUT/kafka_sslkey_creds"
chmod 644 "$OUT/kafka_truststore_creds"

cat "$OUT/mongo.pem" "$OUT/mongo.key" > "$OUT/mongo.server.pem"
chmod 600 "$OUT/mongo.server.pem"

MONGO_KEYFILE="$OUT/mongo.keyfile"

openssl rand -base64 512 | tr -d '\n\r' | head -c 768 > "$MONGO_KEYFILE"
printf '\n' >> "$MONGO_KEYFILE"

chmod 600 "$MONGO_KEYFILE"

echo "OK:"
echo "  CA:              $CA_PEM"
echo "  COMPUTE CERT:    $OUT/compute.pem"
echo "  COMPUTE KEY:     $OUT/compute.key"
echo "  FRONTEND CERT:   $OUT/frontend.pem"
echo "  FRONTEND KEY:    $OUT/frontend.key"
echo "  POSTGRES CERT:   $OUT/db.pem"
echo "  POSTGRES KEY:    $OUT/db.key"
echo "  REDIS CERT:      $OUT/redis.pem"
echo "  REDIS KEY:       $OUT/redis.key"
echo "  MONGO CERT:      $OUT/mongo.pem"
echo "  MONGO KEY:       $OUT/mongo.key"
echo "  MONGO SERVER PEM $OUT/mongo.server.pem"
echo "  MONGO KEYFILE:   $OUT/mongo.keyfile"
echo "  FRONTEND SAN:    $FRONTEND_TLS_HOSTS"
echo "  KAFKA CERT:      $OUT/kafka.pem"
echo "  KAFKA KEY:       $OUT/kafka.key"
echo "  KAFKA KEYSTORE:  $OUT/kafka.keystore.p12"
echo "  KAFKA TRUSTSTORE $OUT/kafka.truststore.p12"
echo "  KAFKA KEYSTORE CREDS:   $OUT/kafka_keystore_creds"
echo "  KAFKA SSLKEY CREDS:     $OUT/kafka_sslkey_creds"
echo "  KAFKA TRUSTSTORE CREDS: $OUT/kafka_truststore_creds"