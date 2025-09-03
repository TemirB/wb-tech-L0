# WB Tech L0 — сервис заказов (Kafka → Postgres → Cache → HTTP + UI)

Демонстрационный микросервис на Go, который:
- потребляет сообщения о заказах из **Kafka**;
- сохраняет данные в **PostgreSQL**;
- кэширует последние заказы в **памяти** для быстрого чтения;
- поднимает **HTTP API** `GET /order/{order_uid}` и простой **веб-интерфейс** для поиска заказа по `order_uid`.

> Реализовано в рамках тестового задания WB Tech L0.

---

## Содержание
- [Архитектура](#архитектура)
- [Стек и требования](#стек-и-требования)
- [Структура репозитория](#структура-репозитория)
- [Варианты запуска](#варианты-запуска)
  - [Быстрый старт через Docker Compose](#быстрый-старт-через-docker-compose)
  - [Локальный запуск (без Docker)](#локальный-запуск-без-docker)
- [Переменные окружения](#переменные-окружения)
- [Схема данных и миграции](#схема-данных-и-миграции)
- [Формат сообщения в Kafka](#формат-сообщения-в-kafka)
- [HTTP API](#http-api)
- [Веб-интерфейс](#веб-интерфейс)
- [Проверка работы и тестовые данные](#проверка-работы-и-тестовые-данные)
- [Кэширование и восстановление](#кэширование-и-восстановление)
---

## Архитектура

```
Kafka (topic: orders) --> Go consumer --> Postgres
                                   \--> cache (LRU)

HTTP API (GET /order/{order_uid}) --> читает из cache, fallback в Postgres
Web UI (HTML/JS) ------------------> дергает API и показывает заказ
```

- **Online-обработка:** потребляем сообщения в реальном времени.
- **Кэш:** ускоряет повторные запросы; при старте сервис прогревает кэш из БД.
- **Надёжность:** при отсутствии записи в кэше читаем из БД и обновляем кэш.

---

## Стек и требования

- Go ≥ 1.21  
- PostgreSQL ≥ 14  
- Kafka ≥ 3.x  
- Docker / Docker Compose
- `make` (опционально, для удобства)

---

## Структура репозитория

```
.
├── cmd/                # точка входа приложения (main)
├── internal/           # http, kafka, storage, cache, models и т.д.
├── migrations/         # SQL-миграции для Postgres
├── docker/             # docker-compose.yml и сопутствующие файлы
├── env/                # .env
├── Makefile            # удобные команды
├── go.mod / go.sum
└── README.md
```
---
## Варианты запуска

### Быстрый старт

1) Соберите и поднимите докер контейнеры, используя make:
```bash
make build
```

2) (Опционально) Запустите спаммер, для проверки работы сервиса и инфраструктуры:
```bash
make spam
```

### Docker Compose

1) Скопируйте пример окружения:
```bash
cp env/.env.example .env
```

2) Поднимите инфраструктуру и сервис:
```bash
docker compose -f docker/docker-compose.yml up -d --build
```

3) Примените миграции (если не применяются автоматически):
```bash
# Вариант через Makefile
make migrate-up
# или напрямую через psql внутри контейнера:
docker compose exec postgres psql -U $POSTGRES_USER -d $POSTGRES_DB -f /migrations/001_init.sql
```

4) Проверьте доступность:
- UI: http://localhost:8081/  
- API: `GET http://localhost:8081/order/<order_uid>`

> Порты и параметры задаются через переменные окружения (см. ниже).

### Локальный запуск (без Docker)

1) Поднимите локально **Postgres** и **Kafka**.  
2) Создайте БД и примените миграции из `migrations/`.  
3) Заполните `.env`.  
4) Запустите сервис:
```bash
go mod download
go run ./cmd/app
```

---

## Переменные окружения

Пример `.env`:
```env
# HTTP
HTTP_ADDR=:8081
CACHE_CAP=1000

# Postgres
PG_HOST=postgres
PG_PORT=5432
PG_DB=orders
PG_USER=app
PG_PASSWORD=app
PG_SSLMODE=disable

# Схема и таблицы
DB_SCHEMA=orders
TBL_ORDER=order
TBL_DELIVERY=delivery
TBL_PAYMENT=payment
TBL_ITEM=item

# Kafka
KAFKA_BROKERS=kafka:9092
KAFKA_TOPIC=orders
KAFKA_GROUP=orders-consumer
KAFKA_WORKERS=10

# Breaker
BREAKER_THRESHOLD=5
BREAKER_OPENTIMEOUT=10000 # ms
BREAKER_MAXHALFOPEN=3

# Retry Policy
RETRY_ATTEMPTS=5
RETRY_BASE=100 # ms
RETRY_MAX=5000 #ms
RETRY_JITTERFACTOR=0.3 # fraction
```

---

## Схема данных и миграции

Данные заказа хранятся в Postgres по модели задания.
- **Нормализованные**: таблицы `orders`, `deliveries`, `payments`, `items` (связь по `order_uid`);

Смотрите SQL в каталоге `migrations/` (например, `001_init.sql`).  
Не забудьте индексы (по `order_uid`, а также по полям JSONB при необходимости).

---

## Формат сообщения в Kafka

Ожидается JSON (пример):

```json
{
  "order_uid": "b563feb7b2b84b6test",
  "track_number": "WBILMTESTTRACK",
  "entry": "WBIL",
  "delivery": {
    "name": "Test Testov",
    "phone": "+972-000-00-00",
    "zip": "2639809",
    "city": "Kiryat Mozkin",
    "address": "Ploshad Mira 15",
    "region": "Kraiot",
    "email": "test@gmail.com"
  },
  "payment": {
    "transaction": "b563feb7b2b84b6test",
    "request_id": "",
    "currency": "USD",
    "provider": "wbpay",
    "amount": 1817,
    "payment_dt": 1637907727,
    "bank": "alpha",
    "delivery_cost": 1500,
    "goods_total": 317,
    "custom_fee": 0
  },
  "items": [
    {
      "chrt_id": 9934930,
      "track_number": "WBILMTESTTRACK",
      "price": 453,
      "rid": "ab4219087a764ae0btest",
      "name": "Mascaras",
      "sale": 30,
      "size": "0",
      "total_price": 317,
      "nm_id": 2389212,
      "brand": "Vivienne Sabo",
      "status": 202
    }
  ],
  "locale": "en",
  "internal_signature": "",
  "customer_id": "test",
  "delivery_service": "meest",
  "shardkey": "9",
  "sm_id": 99,
  "date_created": "2021-11-26T06:22:19Z",
  "oof_shard": "1"
}
```

---

## HTTP API

- `GET /order/{order_uid}` — возвращает JSON заказа.  
  Источник — **кэш**; при отсутствии — **Postgres** (и пополнение кэша).

Примеры:
```bash
curl http://localhost:8081/order/b563feb7b2b84b6test
# -> 200 OK + JSON заказа

# Если не найдено:
# 404 Not Found
# {"error":"order not found"}
```
---

## Веб-интерфейс

Простая страница (HTML/JS) позволяет ввести `order_uid` и получить данные,
обращаясь к `GET /order/{order_uid}`.

Открыть в браузере: **http://localhost:8081/**

---

## Проверка работы и тестовые данные

### Отправка сообщения в Kafka

**Вариант 1 — kafka-console-producer (в контейнере):**
```bash
make spam
или
make spam-fast
или
make spam-heavy
```

**Вариант 2 — API:**
Послать запрос на эндпойнт 
- POST /order/


После получения сообщения сервис:
1) парсит JSON;  
2) сохраняет данные в Postgres;  
3) кладёт заказ в кэш.  
---

## Кэширование и восстановление
- В памяти хранится **последние N** заказов (`CACHE_CAP`).
- При **старте** сервис прогревает кэш **из БД**.
- При **перезапуске** кэш восстанавливается, данные не теряются (берутся из Postgres).