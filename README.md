# secure-voting

## Setup

The .env is not commited to .git, but .env.example is provided.

## Profiles

- prod - core services, ready for production
- debug - prod services + webui's for debugging
- frontend - frontend-only
- backend - backend-only
- compute - compute-only
- db - all db's + redis

## Launching

**Full dev stack**
```shell
docker compose --profile prod --profile debug up -d
```

**Backend-only**
```shell
docker compose --profile backend --profile db up -d
```

## Ports

Containers communicate through internal docker networks.

Only frontend and webui api's are exposed:

|      Service |                 URL |
| ------------ | ------------------- |
|     Frontend |     localhost:8080  |
|      pgAdmin |     localhost:15433 |
|   RedInsight |     localhost:5540  |
| MongoExpress |     localhost:8082  |
| Kafka-Web-UI |     localhost:8089  |


## Service communication

Services communicate through internal docker networks

|  Service | Network |  Port |
| -------- | ------- | ----- |
| Postgres |      db |  5432 |
|    Redis |   redis |  6379 |
|    Mongo |   mongo | 27017 |
|    Kafka |   kafka |  9092 |


## Dev access

You should only access database, if necessary, using `docker exec -it <service-name> <command>`, there are no exposed ports
