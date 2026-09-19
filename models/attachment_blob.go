package models

import (
	"database/sql"
	"errors"
	"time"

	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
)

// ---------------------------------------------------------------------------
// Attachment binary storage in the database (issue #34 reopened).
// Binaries live in the attachment_blobs table (one LONGBLOB row per
// attachment), in a separate table so gallery/list queries never load them.
// The database is the only storage — nothing is written to disk.
// ---------------------------------------------------------------------------

// AttachmentBlob is the binary content of an Attachment.
type AttachmentBlob struct {
	AttachmentID uuid.UUID `json:"attachment_id" db:"attachment_id"`
	Data         []byte    `json:"-" db:"data"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// StoreAttachmentBlob inserts (or replaces) the binary content of an
// attachment inside the caller's transaction. Idempotent per attachment.
func StoreAttachmentBlob(tx *pop.Connection, attachmentID uuid.UUID, data []byte) error {
	now := time.Now()
	return tx.RawQuery(
		`INSERT INTO attachment_blobs (attachment_id, data, created_at, updated_at)
		 VALUES (?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE data = VALUES(data), updated_at = VALUES(updated_at)`,
		attachmentID, data, now, now,
	).Exec()
}

// LoadAttachmentBlob returns the stored binary content of an attachment.
// found=false means the attachment has no blob row (404 on serve).
func LoadAttachmentBlob(tx *pop.Connection, attachmentID uuid.UUID) (data []byte, found bool, err error) {
	b := &AttachmentBlob{}
	q := tx.RawQuery(
		`SELECT attachment_id, data, created_at, updated_at
		 FROM attachment_blobs WHERE attachment_id = ?`, attachmentID)
	if err := q.First(b); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return b.Data, true, nil
}

// DeleteAttachmentBlob removes the binary content row of an attachment
// inside the caller's transaction (no-op when absent).
func DeleteAttachmentBlob(tx *pop.Connection, attachmentID uuid.UUID) error {
	return tx.RawQuery(`DELETE FROM attachment_blobs WHERE attachment_id = ?`, attachmentID).Exec()
}
