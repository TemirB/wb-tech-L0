-- dodelat
CREATE SCHEMA IF NOT EXISTS orders;

CREATE TABLE IF NOT EXISTS orders."order" (
  order_uid TEXT PRIMARY KEY,
  track_number TEXT NOT NULL,
  entry TEXT,
  locale TEXT,
  internal_signature TEXT,
  customer_id TEXT,
  delivery_service TEXT,
  shardkey TEXT,
  sm_id INT,
  date_created TIMESTAMPTZ NOT NULL,
  oof_shard TEXT
);

CREATE TABLE IF NOT EXISTS orders.delivery (
  order_uid TEXT PRIMARY KEY REFERENCES orders."order"(order_uid) ON DELETE CASCADE,
  name TEXT,
  phone TEXT,
  zip TEXT,
  city TEXT,
  address TEXT,
  region TEXT,
  email TEXT
);

CREATE TABLE IF NOT EXISTS orders.payment (
  transaction TEXT PRIMARY KEY,
  order_uid TEXT NOT NULL REFERENCES orders."order"(order_uid) ON DELETE CASCADE,
  request_id TEXT,
  currency TEXT,
  provider TEXT,
  amount INT,
  payment_dt BIGINT,
  bank TEXT,
  delivery_cost INT,
  goods_total INT,
  custom_fee INT
);

CREATE TABLE IF NOT EXISTS orders.item (
  order_uid TEXT NOT NULL REFERENCES orders."order"(order_uid) ON DELETE CASCADE,
  chrt_id INT,
  track_number TEXT,
  price INT,
  rid TEXT,
  name TEXT,
  sale INT,
  size TEXT,
  total_price INT,
  nm_id INT,
  brand TEXT,
  status INT
);

CREATE INDEX IF NOT EXISTS idx_order_date ON orders."order"(date_created DESC);
CREATE INDEX IF NOT EXISTS idx_order_track ON orders."order"(track_number);
CREATE INDEX IF NOT EXISTS idx_item_order ON orders.item(order_uid);
