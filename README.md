# secure-voting

## Description

Secure-voting is a self-hostable platform to host elections and
perform experiments on real or synthetic data across different algorithms.

## Quick Start

```shell
./install.sh              # full setup (generate certs, build and deploy system)
./install.sh --with-debug # launch with admin UIs
./launch.sh               # quick rebuild and restart
./uninstall.sh            # stop all containers and remove their data volumes
```

Then, the web interface becomes available at `https://localhost:8080`.

## Configuration

`.env` file contains all the necessary configuration info for the cluster.
`.env` is not present in the repository, but an example `.env.example` is provided.

## Architecture

```mermaid
flowchart TD
    Browser["Browser"]

    subgraph FE["Frontend"]
        Nginx["nginx (TLS)"]
    end

    subgraph BE["Backend (Go)"]
        API["API Server<br/>:3001"]
        Worker["Worker"]
        CR["Compute Runner"]
    end

    subgraph CS["Compute (Rust)"]
        GRPC["gRPC Service<br/>:50051"]
    end

    subgraph DL["Data Layer"]
        PG[("PostgreSQL<br/>:5432")]
        MG[("MongoDB<br/>:27017")]
        RD[("Redis (TLS)<br/>:6380")]
        KF[("Kafka (TLS)<br/>:29092")]
    end

    Browser -->|HTTPS :443| Nginx
    Nginx -->|HTTP :3001| API

    API --- PG
    API --- RD
    API --- MG
    API -.->|publish tasks| KF

    CR -.->|consume tasks| KF
    CR -->|gRPC (TLS)| GRPC
    CR -.->|publish results| KF

    Worker -.->|consume results| KF
    Worker --- PG
    Worker --- MG
    Worker --- RD
```

## Tech Stack

|  Component |                    Language/Tech |
| ---------- | -------------------------------- |
|    Backend |                               Go |
|   Frontend |               TypeScript + React |
|    Compute |                             Rust |
| Data Layer | Postgres + Mongo + Redis + Kafka |

## Profiles

- prod - core services, ready for production
- debug - prod services + web UIs for debugging
- frontend - frontend-only
- backend - backend-only (Go services)
- compute - compute-only (Rust service)
- db - all db's + redis
- kafka - launch kafka queue

## Ports

Containers communicate through internal docker networks.

Only frontend and webui api's are exposed:

|      Service |                 URL |
| ------------ | ------------------- |
|     Frontend |     localhost:8080  |
|      pgAdmin |     localhost:15433 |
| RedisInsight |     localhost:5540  |
| MongoExpress |     localhost:8082  |
| Kafka-Web-UI |     localhost:8089  |


## Service communication

Services communicate through internal docker networks

|  Service |         Network |  Port |
| -------- | --------------- | ----- |
| Postgres |      db_network |  5432 |
|    Redis |   redis_network |  6380 |
|    Mongo |      db_network | 27017 |
|    Kafka |   kafka_network | 29092 |

## Testing

To run tests, you can use moon's task mechanism and run various tests, for example:

```shell
moon run backend:test # for backend tests
moon run compute:test # for compute tests
moon run e2e:test     # Playwright e2e tests
moon run load:test    # load smoke tests
```

## Replication

There are also docker compose files aimed to provide replication to
postgres, redis and mongo.

```
docker-compose.prod-replication.yml   — full production with HA
docker-compose.postgres-replication.yml
docker-compose.mongo-replication.yml
docker-compose.redis-replication.yml
```

## CI

Github Actions check various things:

- linting
- formatting
- documentation fullness
- correctness (through tests)
- correctness (through compilation success)

## TLS / Certs

All components use TLS certs to communicate inside docker compose network.
`scripts/certs/generate.sh` can be used to generate
self-signed development certificates if you don'st have your own.
