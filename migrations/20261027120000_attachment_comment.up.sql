-- Bug 2026-10-27 #7 (second review batch): media upload gains a
-- comment/details field — set at upload, editable by the uploader/admin.
ALTER TABLE attachments ADD COLUMN `comment` VARCHAR(500) NULL;
