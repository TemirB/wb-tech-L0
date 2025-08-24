package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/TemirB/wb-tech-L0/internal/config"
	"github.com/TemirB/wb-tech-L0/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repo struct {
	pool   *pgxpool.Pool
	tables config.Tables
}

func New(pool *pgxpool.Pool, t config.Tables) *Repo { return &Repo{pool: pool, tables: t} }

func Connect(ctx context.Context, dsn string) *pgxpool.Pool {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		panic(err)
	}
	if err := pool.Ping(ctx); err != nil {
		panic(err)
	}
	return pool
}

func (r *Repo) qt(tbl string) string { return fmt.Sprintf(`"%s"."%s"`, r.tables.Schema, tbl) }

func (r *Repo) Upsert(ctx context.Context, o *domain.Order) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s (order_uid, track_number, entry, locale, internal_signature,
		  customer_id, delivery_service, shardkey, sm_id, date_created, oof_shard)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (order_uid) DO UPDATE SET
		  track_number=EXCLUDED.track_number,
		  entry=EXCLUDED.entry,
		  locale=EXCLUDED.locale,
		  internal_signature=EXCLUDED.internal_signature,
		  customer_id=EXCLUDED.customer_id,
		  delivery_service=EXCLUDED.delivery_service,
		  shardkey=EXCLUDED.shardkey,
		  sm_id=EXCLUDED.sm_id,
		  date_created=EXCLUDED.date_created,
		  oof_shard=EXCLUDED.oof_shard
	`, r.qt(r.tables.Order)),
		o.OrderUID, o.TrackNumber, o.Entry, o.Locale, o.InternalSignature, o.CustomerID,
		o.DeliveryService, o.ShardKey, o.SmID, o.DateCreated, o.OofShard,
	)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s (order_uid, name, phone, zip, city, address, region, email)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (order_uid) DO UPDATE SET
		  name=EXCLUDED.name, phone=EXCLUDED.phone, zip=EXCLUDED.zip, city=EXCLUDED.city,
		  address=EXCLUDED.address, region=EXCLUDED.region, email=EXCLUDED.email
	`, r.qt(r.tables.Delivery)),
		o.OrderUID, o.Delivery.Name, o.Delivery.Phone, o.Delivery.Zip, o.Delivery.City,
		o.Delivery.Address, o.Delivery.Region, o.Delivery.Email,
	)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s (transaction, order_uid, request_id, currency, provider, amount,
		  payment_dt, bank, delivery_cost, goods_total, custom_fee)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (transaction) DO UPDATE SET
		  order_uid=EXCLUDED.order_uid, request_id=EXCLUDED.request_id, currency=EXCLUDED.currency,
		  provider=EXCLUDED.provider, amount=EXCLUDED.amount, payment_dt=EXCLUDED.payment_dt, bank=EXCLUDED.bank,
		  delivery_cost=EXCLUDED.delivery_cost, goods_total=EXCLUDED.goods_total, custom_fee=EXCLUDED.custom_fee
	`, r.qt(r.tables.Payment)),
		o.Payment.Transaction, o.OrderUID, o.Payment.RequestID, o.Payment.Currency, o.Payment.Provider,
		o.Payment.Amount, o.Payment.PaymentDT, o.Payment.Bank, o.Payment.DeliveryCost,
		o.Payment.GoodsTotal, o.Payment.CustomFee,
	)
	if err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s WHERE order_uid=$1`, r.qt(r.tables.Item)), o.OrderUID); err != nil {
		return err
	}
	batch := &pgx.Batch{}
	for _, it := range o.Items {
		batch.Queue(fmt.Sprintf(`
			INSERT INTO %s (order_uid, chrt_id, track_number, price, rid, name, sale, size, total_price, nm_id, brand, status)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		`, r.qt(r.tables.Item)),
			o.OrderUID, it.ChrtID, it.TrackNumber, it.Price, it.RID, it.Name, it.Sale,
			it.Size, it.TotalPrice, it.NmID, it.Brand, it.Status,
		)
	}
	if br := tx.SendBatch(ctx, batch); br != nil {
		if err := br.Close(); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *Repo) GetByUID(ctx context.Context, uid string) (*domain.Order, error) {
	var o domain.Order
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT order_uid, track_number, entry, locale, internal_signature, customer_id, delivery_service,
		       shardkey, sm_id, date_created, oof_shard
		FROM %s WHERE order_uid=$1
	`, r.qt(r.tables.Order)), uid).Scan(
		&o.OrderUID, &o.TrackNumber, &o.Entry, &o.Locale, &o.InternalSignature, &o.CustomerID,
		&o.DeliveryService, &o.ShardKey, &o.SmID, &o.DateCreated, &o.OofShard,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT name, phone, zip, city, address, region, email
		FROM %s WHERE order_uid=$1
	`, r.qt(r.tables.Delivery)), uid).Scan(
		&o.Delivery.Name, &o.Delivery.Phone, &o.Delivery.Zip, &o.Delivery.City,
		&o.Delivery.Address, &o.Delivery.Region, &o.Delivery.Email,
	); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	if err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT transaction, request_id, currency, provider, amount, payment_dt, bank, delivery_cost, goods_total, custom_fee
		FROM %s WHERE order_uid=$1
	`, r.qt(r.tables.Payment)), uid).Scan(
		&o.Payment.Transaction, &o.Payment.RequestID, &o.Payment.Currency, &o.Payment.Provider,
		&o.Payment.Amount, &o.Payment.PaymentDT, &o.Payment.Bank, &o.Payment.DeliveryCost,
		&o.Payment.GoodsTotal, &o.Payment.CustomFee,
	); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT chrt_id, track_number, price, rid, name, sale, size, total_price, nm_id, brand, status
		FROM %s WHERE order_uid=$1
	`, r.qt(r.tables.Item)), uid)
	if err != nil {
		return &o, err
	}
	defer rows.Close()

	for rows.Next() {
		var it domain.Item
		if err := rows.Scan(&it.ChrtID, &it.TrackNumber, &it.Price, &it.RID, &it.Name, &it.Sale,
			&it.Size, &it.TotalPrice, &it.NmID, &it.Brand, &it.Status); err != nil {
			return nil, err
		}
		o.Items = append(o.Items, it)
	}
	return &o, rows.Err()
}

func (r *Repo) RecentOrderIDs(ctx context.Context, limit int) ([]string, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT order_uid FROM %s
		ORDER BY date_created DESC NULLS LAST
		LIMIT $1
	`, r.qt(r.tables.Order)), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
