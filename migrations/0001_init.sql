CREATE ROLE orders_user LOGIN PASSWORD 'orders_pass';
CREATE DATABASE ordersdb OWNER orders_user;

\connect ordersdb

CREATE SCHEMA IF NOT EXISTS orders AUTHORIZATION orders_user;
SET search_path TO orders;

CREATE TABLE orders."order" (
  order_uid        text PRIMARY KEY,
  track_number     text NOT NULL,
  entry            text NOT NULL,
  locale           text,
  internal_signature text,
  customer_id      text,
  delivery_service text,
  shardkey         text,
  sm_id            integer,
  date_created     timestamptz NOT NULL,
  oof_shard        text
);

CREATE TABLE orders.delivery (
  order_uid  text PRIMARY KEY REFERENCES orders."order"(order_uid) ON DELETE CASCADE,
  name       text, phone text, zip text, city text,
  address    text, region text, email text
);

CREATE TABLE orders.payment (
  order_uid     text PRIMARY KEY REFERENCES orders."order"(order_uid) ON DELETE CASCADE,
  transaction   text NOT NULL UNIQUE,
  request_id    text,
  currency      text,
  provider      text,
  amount        integer,
  payment_dt    bigint,
  bank          text,
  delivery_cost integer,
  goods_total   integer,
  custom_fee    integer
);

CREATE TABLE orders.item (
  id           bigserial PRIMARY KEY,
  order_uid    text NOT NULL REFERENCES orders."order"(order_uid) ON DELETE CASCADE,
  chrt_id      integer,
  track_number text,
  price        integer,
  rid          text,
  name         text,
  sale         integer,
  size         text,
  total_price  integer,
  nm_id        integer,
  brand        text,
  status       integer
);

CREATE INDEX ON orders."order" (date_created DESC);
CREATE INDEX ON orders.item (order_uid);

GRANT USAGE ON SCHEMA orders TO orders_user;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA orders TO orders_user;
ALTER DEFAULT PRIVILEGES IN SCHEMA orders GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO orders_user;