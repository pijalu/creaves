## #34 (enhancement) — Pièces jointes (photos/vidéos) — DONE ✅ commit 73662d9
**Source:** https://github.com/pijalu/creaves/issues/34
**Observed:** No attachment support on animal.
**Expected:** Attach photos/videos to animal record; viewable in animal fiche.
**Plan:**
1. Schema (additive): `attachments` table (id, animal_id, filename, content_type, size, storage_path, uploaded_by, created_at).
2. Storage: local disk dir per instance (config path) — instance is per-center, no S3 dependency needed; enforce size/type whitelist (jpg/png/mp4 ≤ N MB).
3. Actions: upload (multipart), download/serve, delete (creator/admin only). Buffalo multipart handling.
4. Template: gallery block in animal fiche; thumbnail for images.
5. Security: auth same as animal view; filename sanitization; content-type check.
**Test:** unit upload/delete + auth + oversize/wrong-type rejection; e2e upload → visible → delete.
**Validation:** agent-browser upload flow; disk state checked; `go test ./...`.
**Implemented (commit 73662d9):** `attachments` metadata table (uuid pk, animal_id, filename, content_type, size, kind, uploaded_by); binaries under `attachments-storage/animal-<id>/<uuid>.<ext>` (`ATTACHMENTS_DIR` override, gitignored). Whitelist enforced by content SNIFFING (jpeg/png/webp/gif, mp4/webm) with declared-type fallback; svg/pdf/html rejected; limits 10 MB images / 100 MB videos enforced twice (header + LimitedReader at store time). `POST /animals/{animal_id}/attachments` (any user on animals in care, admin on outtaken — mirrors care rules), `GET /attachments/{id}` inline via http.ServeContent (range → video seeking, Content-Type pinned, disposition filename sanitized), `POST /attachments/{id}/delete` uploader-or-admin (`isOwner := UploadedBy.Valid && == user.ID` — invalid/legacy uploader rows = admin only). Audit entries with new `AuditEntityAttachment`. Gallery = new "Media/Médias/Medien" tab on animal fiche ×4 locales: image thumbnails, inline `<video>`, per-card delete with confirm, localized empty state + flash keys (`locales/attachments.*.yaml`).
**Test:** `GO_ENV=test go test -run TestAttachment ./models/ ./actions/` — kind whitelist, validate (type/size/path), SanitizeFilename (traversal, control chars, length cap); handler upload→serve→delete roundtrip (byte-identical, content-type, disposition), wrong-type + oversize rejection (limit lowered in test via package var), ownership 403 vs owner/admin delete.
**Validation:** agent-browser e2e — real PNG upload through the form → card in gallery, served bytes/type verified in-session, file on disk under uuid path, delete → row 0 + file gone + empty state restored. Test artifacts removed.
