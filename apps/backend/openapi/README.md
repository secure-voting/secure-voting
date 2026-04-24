# secure-voting OpenAPI

В файле `openapi.yaml` находится описание REST API backend.

Правила поддержки:
1. Любое изменение публичного API сначала отражается в `openapi.yaml`.
2. Если меняется request/response/error contract, одновременно обновляются backend handlers/services и frontend client/types.
3. Для новых endpoint'ов обязательно описывать:
   - security requirements,
   - request body / query params,
   - response schemas,
   - стандартные error responses.
4. Для критичных endpoint'ов, таких как `auth`, `ballots/submit`, `results`, `experiment-runs`, изменения должны сопровождаться тестами.

Минимальный локальный тест для проверки минимальной корректности:
- обновить `openapi.yaml`
- прогнать `go test ./...`
- прогнать `go vet ./...`
- прогнать `npm run typecheck`

Спецификация пока ведётся вручную и уточняется по мере стабилизации контрактов.