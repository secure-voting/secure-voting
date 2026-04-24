unset COMPOSE_FILE
unset COMPOSE_PROFILES
unset COMPOSE_PROJECT_NAME

docker compose down -v --remove-orphans || true

docker ps -a --format '{{.ID}} {{.Names}}' | grep 'secure-voting\|postgres-db\|kafka' || true

docker ps -aq --filter "name=go-backend" | xargs -r docker rm -f
docker ps -aq --filter "name=go-worker" | xargs -r docker rm -f
docker ps -aq --filter "name=go-compute-runner" | xargs -r docker rm -f
docker ps -aq --filter "name=ts-frontend" | xargs -r docker rm -f
docker ps -aq --filter "name=rust-compute" | xargs -r docker rm -f
docker ps -aq --filter "name=postgres-db" | xargs -r docker rm -f
docker ps -aq --filter "name=kafka" | xargs -r docker rm -f
docker ps -aq --filter "name=mongo-db" | xargs -r docker rm -f
docker ps -aq --filter "name=redis-cache" | xargs -r docker rm -f

docker volume ls --format '{{.Name}}' | grep '^secure-voting' | xargs -r docker volume rm -f

rm -rf scripts/certs/out
mkdir -p scripts/certs/out

bash scripts/certs/generate.sh

export COMPOSE_PROFILES=prod
docker compose up -d --build

docker compose ps
