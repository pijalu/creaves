## #34 (reopened) — Attachments: binaries stored in DB — DONE ✅ commit 158d150
**Source:** https://github.com/pijalu/creaves/issues/34
**Original:** attachments feature (photos/videos) shipped in commit 73662d9 with
binaries on the local disk under `attachments-storage/` (see
2026-09-19-issue-34-attachments.md).
**Reopened (maintainer):** storage must be in the DB for the file — no legacy,
DB is the only approach (no disk fallback, no migration task).
**Plan:**
1. Additive schema: `attachment_blobs` (attachment_id char(36) PK, data LONGBLOB
   NOT NULL, timestamps, FK -> attachments ON DELETE CASCADE). Separate table so
   gallery/list queries never load blobs. Raw SQL migration — fizz "blob" maps to
   MySQL BLOB (64 KB), too small for 100 MB videos.
2. models: `AttachmentBlob` + `StoreAttachmentBlob` (idempotent upsert) /
   `LoadAttachmentBlob` (found=false when absent) / `DeleteAttachmentBlob`.
3. Actions: upload reads bytes (LimitReader, double limit check) and stores the
   blob in the SAME transaction as the row insert (failure rolls back both);
   serve streams from the DB via `http.ServeContent` (range requests → video
   seeking preserved), 404 when no blob; delete removes blob + row in one tx.
   `storage_path` kept populated as logical locator (audit), nothing on disk;
   ATTACHMENTS_DIR / attachmentsRoot removed entirely.
**Deploy note:** MariaDB/MySQL `max_allowed_packet` must exceed the biggest
allowed upload (256M recommended) — documented in the migration header; blob
insert failure logs a hint and returns an error (row rolls back).
**Test:** `GO_ENV=test go test -run TestAttach ./actions/ ./models/` —
upload→blob byte-identical + NoFileExists(disk path), serve from DB
(content-type/disposition/bytes), missing-blob 404, upsert re-store (1 row,
replaced bytes), ownership 403/owner delete removes row+blob, type/size
rejection. Full suite `-race -cover` green except the documented pre-existing
grifts baseline failure (outtaketype seed name-collision), untouched by this
change (migration replays cleanly).
**Validation (agent-browser e2e, dev server, admin/admin, animal 980492):**
login → Media tab (empty) → upload /tmp/e2e-attach.png (70-byte PNG) →
flash success + gallery card `e2e-attach.png`; DB check
`SELECT attachment_id, LENGTH(data) FROM attachment_blobs` →
`de03408d-a024-47ca-bf22-ac0ca613a843 | 70`; `ls attachments-storage` →
No such file or directory (nothing on disk); navigate serve URL → renders
`image/png (1×1)`, session fetch → `len=70 magic=137,80,78,71,13,10,26,10`
(PNG magic); card Delete → confirm dialog accepted → flash "Attachment
deleted." → gallery back to empty upload form; DB → attachments=0, blobs=0
(pre-test values restored). Artifacts /tmp files only, browser closed.
