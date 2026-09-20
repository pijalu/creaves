package models

import (
	"time"

	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
)

// MaxDeliveryAttempts caps the number of delivery attempts per event and
// sync target before the delivery is considered undeliverable.
const MaxDeliveryAttempts = 25

// EventDelivery tracks the delivery state of one event stream entry towards
// one sync target. The legacy aggregate columns on event_streams
// (delivered_at, acknowledged_at, delivery_attempts, last_delivery_error)
// are kept in sync as a rollup over all enabled targets: delivered once
// every enabled target has been delivered, acknowledged once every enabled
// target has acknowledged.
type EventDelivery struct {
	ID             uuid.UUID  `json:"id" db:"id"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at" db:"updated_at"`
	EventID        uuid.UUID  `json:"event_id" db:"event_id"`
	TargetID       uuid.UUID  `json:"target_id" db:"target_id"`
	Attempts       int        `json:"attempts" db:"attempts"`
	DeliveredAt    *time.Time `json:"delivered_at,omitempty" db:"delivered_at"`
	AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty" db:"acknowledged_at"`
	LastError      *string    `json:"last_error,omitempty" db:"last_error"`
}

// EventDeliveries is a convenience slice type.
type EventDeliveries []EventDelivery

// Undeliverable reports whether this delivery exhausted its retry budget.
func (d *EventDelivery) Undeliverable() bool {
	return d.Attempts >= MaxDeliveryAttempts
}

// Pending reports whether this delivery still needs a (re)try.
func (d *EventDelivery) Pending() bool {
	return d.DeliveredAt == nil && !d.Undeliverable()
}

// DeliveryCounts aggregates per-target delivery statistics for the admin UI.
type DeliveryCounts struct {
	Pending       int `db:"pending"`
	Delivered     int `db:"delivered"`
	Undeliverable int `db:"undeliverable"`
	Unacked       int `db:"unacked"`
}

// DeliveryCountsForTarget computes delivery statistics for one sync target.
func DeliveryCountsForTarget(tx *pop.Connection, targetID uuid.UUID) (DeliveryCounts, error) {
	var c DeliveryCounts
	q := `SELECT
		COALESCE(SUM(CASE WHEN delivered_at IS NULL AND attempts < ? THEN 1 ELSE 0 END),0) AS pending,
		COALESCE(SUM(CASE WHEN delivered_at IS NOT NULL THEN 1 ELSE 0 END),0) AS delivered,
		COALESCE(SUM(CASE WHEN delivered_at IS NULL AND attempts >= ? THEN 1 ELSE 0 END),0) AS undeliverable,
		COALESCE(SUM(CASE WHEN delivered_at IS NOT NULL AND acknowledged_at IS NULL THEN 1 ELSE 0 END),0) AS unacked
		FROM event_deliveries WHERE target_id = ?`
	err := tx.RawQuery(q, MaxDeliveryAttempts, MaxDeliveryAttempts, targetID.String()).First(&c)
	return c, err
}
