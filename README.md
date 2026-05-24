# Search Trends

Сервис для виджета "Сейчас ищут". Читает поток поисковых событий из Kafka и отдаёт топ популярных поисковых запросов за последние 5 минут через HTTP API.

## Быстрый запуск

```bash
docker compose up --build
```

Проверка, что сервис поднялся:

```bash
curl http://localhost:8080/health
```

Пример ответа:

```json
{"status":"ok"}
```

Отправить несколько поисковых событий в Kafka (через kafka-console-producer внутри контейнера):

```bash
docker compose exec kafka bash -c \
  "echo '{\"query\":\"iphone 15\",\"user_id\":\"u1\",\"request_id\":\"r1\",\"timestamp\":\"'\"\$(date -u +%Y-%m-%dT%H:%M:%SZ)\"'\",\"source\":\"search-api\"}' \
   | kafka-console-producer.sh --bootstrap-server kafka:9092 --topic search.events"

docker compose exec kafka bash -c \
  "echo '{\"query\":\"iphone 15\",\"user_id\":\"u2\"}' \
   | kafka-console-producer.sh --bootstrap-server kafka:9092 --topic search.events"

docker compose exec kafka bash -c \
  "echo '{\"query\":\"sneakers\",\"user_id\":\"u3\"}' \
   | kafka-console-producer.sh --bootstrap-server kafka:9092 --topic search.events"
```

Получить топ запросов:

```bash
curl 'http://localhost:8080/top?limit=10'
```

Пример ответа:

```json
{
  "items": [
    {"query": "iphone 15", "count": 2},
    {"query": "sneakers",  "count": 1}
  ],
  "window": "5m0s"
}
```

## HTTP API

`GET /health` (и `HEAD /health`) — проверка состояния.

`GET /top?limit=10` — топ запросов за окно. `limit` ∈ `[1..1000]`.

`GET /stop-list` — текущий стоп-лист.

`POST /stop-list` — добавить запрос в стоп-лист без перезапуска.

```bash
curl -X POST http://localhost:8080/stop-list \
  -H 'Content-Type: application/json' \
  -d '{"query":"iphone 15"}'
```

`DELETE /stop-list/{query}` — удалить запрос из стоп-листа.

```bash
curl -X DELETE http://localhost:8080/stop-list/iphone%2015
```

`GET /metrics` — базовые метрики в Prometheus-формате (с `# HELP` / `# TYPE`).

## Контракт события

Сервис ожидает JSON-сообщения в Kafka-топике `search.events`.

```json
{
  "query": "iphone 15",
  "user_id": "u123",
  "request_id": "req-456",
  "timestamp": "2026-05-22T12:00:00Z",
  "source": "search-api"
}
```

Обязательное поле:

- `query` — исходный поисковый запрос. Сервис нормализует его в единственном месте — доменный VO `SearchQuery` (`internal/domain/trends/event.go`): trim, lowercase, схлопывание повторяющихся пробелов.

Дополнительные поля:

- `timestamp` — время события. Если пусто или нулевое — берётся текущее время сервиса.
- `user_id` — стабильный идентификатор пользователя (заготовка под per-user антифрод).
- `request_id` — для трассировки и потенциальной идемпотентности.
- `source` — имя сервиса-источника.

## Архитектура

```
cmd/main.go                          composition root + graceful shutdown
internal/
├── domain/trends/                   SearchQuery (VO), TrendEntry, sentinel errors
├── application/trends/              use cases (Ingest, QueryTop, StopList) + порты
├── adapter/
│   ├── http/handler.go              тонкие HTTP-хендлеры
│   └── kafka/consumer.go            Kafka-консьюмер (franz-go)
├── config/                          ENV-загрузка с валидацией
└── infra/
    ├── metrics/prometheus.go        MetricsSink + /metrics handler
    └── trends/memory_store.go       in-memory sliding-window реализация TrendsStore
```

Хранилище — in-memory скользящее окно, разбитое на бакеты по `BUCKET_RESOLUTION` (1 с по умолчанию). Каждый бакет хранит счётчики по нормализованным запросам. Дополнительно держится `totals` — агрегированные счётчики по всему окну, чтобы `Top()` не пересканировал бакеты. Когда бакет устаревает или переиспользуется (ring buffer), его значения вычитаются из `totals`.

Чтений `/top` ожидается значительно больше, чем записей. Поэтому `/top` возвращает копию кешированного отсортированного snapshot, который пересобирается лениво по `dirty`-флагу.

Стоп-лист применяется динамически. После удаления из стоп-листа исторические счётчики снова появляются в топе, пока их события не выйдут из окна.

## Защита от накруток

```bash
MAX_QUERY_PER_BUCKET=1000
```

Лимит одинаковых нормализованных запросов в один временной бакет. При превышении событие отклоняется (`ErrRateLimited`).

## Kafka-консьюмер

- Клиент: [`twmb/franz-go`](https://github.com/twmb/franz-go) — pure Go, без cgo.
- Consumer group + `AutoCommitMarks` с интервалом 5s.
- `MarkCommitRecords` вызывается после обработки **каждого** сообщения (включая malformed JSON и доменные отказы) — poison-сообщения не блокируют партицию.
- Graceful shutdown: SIGINT/SIGTERM → `cancel()` → `wg.Wait()` → финальный `CommitMarkedOffsets` → `client.Close()`.

## Переменные окружения

| Имя | По умолчанию | Назначение |
|---|---|---|
| `HTTP_ADDR` | `:8080` | Адрес HTTP-сервера |
| `KAFKA_BROKERS` | `localhost:9092` | Список брокеров через запятую |
| `KAFKA_TOPIC` | `search.events` | Топик с событиями |
| `KAFKA_GROUP_ID` | `search-trends` | Consumer group ID |
| `WINDOW` | `5m` | Длина окна |
| `BUCKET_RESOLUTION` | `1s` | Размер бакета |
| `MAX_QUERY_PER_BUCKET` | `1000` | Лимит одинаковых запросов в бакет |

## Тесты

```bash
go test -race ./...
```

## Производительность

### Go-бенчмарки

```bash
go test -run=^$ -bench=. -benchmem -benchtime=2s ./internal/infra/trends/
```

Результаты на **Apple M4 Pro** (10k уникальных запросов в окне):

| Бенчмарк | ns/op | allocs/op | B/op | Что измеряет |
|---|---:|---:|---:|---|
| `Store_Add_SameQuery` | 317 | 0 | 0 | Hot-path запись повторов |
| `Store_Add_UniqueQueries` | 371 | 0 | 0 | Запись с card.=10k |
| `Store_Add_Parallel` (12 CPU) | 499 | 0 | 0 | Запись под mutex-contention |
| `Store_Top_Cached` | 309 | 1 | 240 | Чтение из готового snapshot |
| `Store_Top_Dirty` (N=10k) | 1 290 000 | 5 | 246 KB | Полная пересборка snapshot |
| `Store_ReadHeavyMix` 95/5 | 64 000 | 1 | 12 KB | 95% Top, 5% Add (под параллельной нагрузкой) |

**Top по кардинальности** (one Add + one Top на итерацию):

| Unique queries | ns/op | B/op |
|---:|---:|---:|
| 100 | 7 200 | 3 KB |
| 1 000 | 101 000 | 25 KB |
| 10 000 | 1 320 000 | 246 KB |
| 100 000 | 16 515 000 | 2.4 MB |

Из чисел видно:
- **`Add` — это O(1)** с нулевыми аллокациями. Сервис выдержит ~2-3M событий/сек на одно ядро (теоретический потолок без сетевых издержек).
- **`Top` с кэшем — 309 ns**, копия snapshot. Чтения почти бесплатны.
- **`Top` с пересборкой** растёт линейно (`O(N log N)`). При N > ~10k стоит заменить `sort.Slice` на heap top-K или вынести пересборку в фон.

### Нагрузочный тест через `hey`

Скрипт `scripts/loadtest.sh` публикует 10k ивентов в Kafka и затем 30s бьёт `/top` со 100 параллельных воркеров:

```bash
docker compose up -d
brew install hey   # или go install github.com/rakyll/hey@latest
./scripts/loadtest.sh
```

Дополнительные параметры через ENV:

```bash
EVENTS=100000 UNIQUE=200 DURATION=60s CONCURRENCY=500 ./scripts/loadtest.sh
```

### Только публикация ивентов

```bash
./scripts/produce.sh 50000 100      # 50k событий, 100 уникальных запросов
```

### Только чтение

```bash
hey -z 30s -c 100 'http://localhost:8080/top?limit=10'
```
