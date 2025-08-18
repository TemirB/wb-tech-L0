package postgres

import (
	"context"

	"github.com/TemirB/wb-tech-L0/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OrderRepository struct {
	pool *pgxpool.Pool
}

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

func NewOrderRepository(pool *pgxpool.Pool) *OrderRepository {
	return &OrderRepository{pool: pool}
}

func (r *OrderRepository) Upsert(ctx context.Context, o *domain.Order) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// order
	_, err = tx.Exec(ctx, `
		INSERT INTO orders.order (order_uid, track_number, entry, locale, internal_signature,
			customer_id, delivery_service, shardkey, sm_id, date_created, oof_shard)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (order_uid)
		DO UPDATE SET
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
		`, o.OrderUID, o.TrackNumber, o.Entry, o.Locale, o.InternalSignature, o.CustomerID,
		o.DeliveryService, o.ShardKey, o.SmID, o.DateCreated, o.OofShard,
	)
	if err != nil {
		return err
	}

	// delivery
	_, err = tx.Exec(ctx, `
  		INSERT INTO orders.delivery (order_uid, name, phone, zip, city, address, region, email)
  		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (order_uid)
		DO UPDATE SET
    		name=EXCLUDED.name,
			phone=EXCLUDED.phone,
			zip=EXCLUDED.zip,
			city=EXCLUDED.city,
    		address=EXCLUDED.address,
			region=EXCLUDED.region,
			email=EXCLUDED.email
		`, o.OrderUID, o.Delivery.Name, o.Delivery.Phone, o.Delivery.Zip, o.Delivery.City,
		o.Delivery.Address, o.Delivery.Region, o.Delivery.Email)
	if err != nil {
		return err
	}

	// payment
	_, err = tx.Exec(ctx, `
		INSERT INTO orders.payment (order_uid, transaction, request_id, currency, provider, amount,
			payment_dt, bank, delivery_cost, goods_total, custom_fee)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (order_uid) DO UPDATE SET
			transaction=EXCLUDED.transaction,
			request_id=EXCLUDED.request_id,
			currency=EXCLUDED.currency,
			provider=EXCLUDED.provider,
			amount=EXCLUDED.amount,
			payment_dt=EXCLUDED.payment_dt,
			bank=EXCLUDED.bank,
			delivery_cost=EXCLUDED.delivery_cost,
			goods_total=EXCLUDED.goods_total,
			custom_fee=EXCLUDED.custom_fee
		`, o.OrderUID, o.Payment.Transaction, o.Payment.RequestID, o.Payment.Currency, o.Payment.Provider,
		o.Payment.Amount, o.Payment.PaymentDT, o.Payment.Bank, o.Payment.DeliveryCost,
		o.Payment.GoodsTotal, o.Payment.CustomFee)
	if err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `DELETE FROM orders.item WHERE order_uid=$1`, o.OrderUID); err != nil {
		return err
	}
	for _, it := range o.Items {
		_, err := tx.Exec(ctx, `
			INSERT INTO orders.item (order_uid, chrt_id, track_number, price, rid, name, sale, size, total_price, nm_id, brand, status)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
			`, o.OrderUID, it.ChrtID, it.TrackNumber, it.Price, it.RID, it.Name, it.Sale,
			it.Size, it.TotalPrice, it.NmID, it.Brand, it.Status)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *OrderRepository) GetByID(ctx context.Context, uid string) (*domain.Order, error) {
	var o domain.Order
	err := r.pool.QueryRow(ctx, `
		SELECT order_uid, track_number, entry, locale, internal_signature, customer_id, delivery_service, shardkey, sm_id, date_created, oof_shard
		FROM orders.order
		WHERE order_uid=$1
		`, uid).Scan(&o.OrderUID, &o.TrackNumber, &o.Entry, &o.Locale, &o.InternalSignature, &o.CustomerID,
		&o.DeliveryService, &o.ShardKey, &o.SmID, &o.DateCreated, &o.OofShard)
	if err != nil {
		return nil, domain.ErrNotFound
	}

	_ = r.pool.QueryRow(ctx, `
		SELECT name, phone, zip, city, address, region, email
		FROM orders.delivery
		WHERE order_uid=$1
		`, uid).Scan(&o.Delivery.Name, &o.Delivery.Phone, &o.Delivery.Zip, &o.Delivery.City,
		&o.Delivery.Address, &o.Delivery.Region, &o.Delivery.Email)

	_ = r.pool.QueryRow(ctx, `
  		SELECT transaction, request_id, currency, provider, amount, payment_dt, bank, delivery_cost, goods_total, custom_fee
  		FROM orders.payment
		WHERE order_uid=$1
		`, uid).Scan(&o.Payment.Transaction, &o.Payment.RequestID, &o.Payment.Currency, &o.Payment.Provider,
		&o.Payment.Amount, &o.Payment.PaymentDT, &o.Payment.Bank, &o.Payment.DeliveryCost,
		&o.Payment.GoodsTotal, &o.Payment.CustomFee)

	rows, err := r.pool.Query(ctx, `
		SELECT chrt_id, track_number, price, rid, name, sale, size, total_price, nm_id, brand, status
		FROM orders.item
		WHERE order_uid=$1
		`, uid)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var it domain.Item
			if scanErr := rows.Scan(&it.ChrtID, &it.TrackNumber, &it.Price, &it.RID, &it.Name, &it.Sale,
				&it.Size, &it.TotalPrice, &it.NmID, &it.Brand, &it.Status); scanErr == nil {
				o.Items = append(o.Items, it)
			}
		}
	}
	return &o, nil
}

func (r *OrderRepository) RecentOrderIDs(ctx context.Context, limit int) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT order_uid FROM orders.order
		ORDER BY date_created DESC LIMIT $1
		`, limit)
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
