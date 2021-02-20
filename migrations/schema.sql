CREATE TABLE IF NOT EXISTS "schema_migration" (
"version" TEXT NOT NULL
);
CREATE UNIQUE INDEX "schema_migration_version_idx" ON "schema_migration" (version);
CREATE TABLE IF NOT EXISTS "users" (
"id" TEXT PRIMARY KEY,
"email" TEXT NOT NULL,
"admin" bool NOT NULL DEFAULT 'false',
"approved" bool NOT NULL DEFAULT 'false',
"password_hash" TEXT NOT NULL,
"created_at" DATETIME NOT NULL,
"updated_at" DATETIME NOT NULL
);
CREATE TABLE IF NOT EXISTS "logentries" (
"id" TEXT PRIMARY KEY,
"user_id" char(36) NOT NULL,
"description" TEXT NOT NULL,
"created_at" DATETIME NOT NULL,
"updated_at" DATETIME NOT NULL,
FOREIGN KEY (user_id) REFERENCES users (id)
);
CREATE TABLE IF NOT EXISTS "discoverers" (
"id" TEXT PRIMARY KEY,
"firstname" TEXT NOT NULL,
"lastname" TEXT NOT NULL,
"address" TEXT NOT NULL,
"city" TEXT NOT NULL,
"country" TEXT NOT NULL,
"email" TEXT,
"phone" TEXT,
"note" TEXT,
"created_at" DATETIME NOT NULL,
"updated_at" DATETIME NOT NULL
);
CREATE TABLE IF NOT EXISTS "discoveries" (
"id" TEXT PRIMARY KEY,
"location" TEXT NOT NULL,
"date" DATETIME NOT NULL,
"reason" TEXT,
"note" TEXT,
"discoverer_id" char(36) NOT NULL,
"created_at" DATETIME NOT NULL,
"updated_at" DATETIME NOT NULL,
FOREIGN KEY (discoverer_id) REFERENCES discoverers (id) ON DELETE CASCADE
);
