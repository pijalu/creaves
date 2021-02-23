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
CREATE TABLE IF NOT EXISTS "animaltypes" (
"id" TEXT PRIMARY KEY,
"name" TEXT NOT NULL,
"description" TEXT,
"def" bool NOT NULL DEFAULT 'false',
"created_at" DATETIME NOT NULL,
"updated_at" DATETIME NOT NULL
);
CREATE TABLE IF NOT EXISTS "intakes" (
"id" TEXT PRIMARY KEY,
"date" DATETIME NOT NULL,
"general" TEXT NOT NULL,
"wounds" TEXT,
"parasites" TEXT,
"remarks" TEXT,
"created_at" DATETIME NOT NULL,
"updated_at" DATETIME NOT NULL
);
CREATE TABLE IF NOT EXISTS "outtaketypes" (
"id" TEXT PRIMARY KEY,
"name" TEXT NOT NULL,
"description" TEXT,
"def" bool NOT NULL DEFAULT 'false',
"created_at" DATETIME NOT NULL,
"updated_at" DATETIME NOT NULL
);
CREATE TABLE IF NOT EXISTS "outtakes" (
"id" TEXT PRIMARY KEY,
"date" DATETIME NOT NULL,
"outtaketype_id" char(36) NOT NULL,
"location" TEXT,
"note" TEXT,
"created_at" DATETIME NOT NULL,
"updated_at" DATETIME NOT NULL,
FOREIGN KEY (outtaketype_id) REFERENCES outtaketypes (id)
);
CREATE TABLE IF NOT EXISTS "animals" (
"id" INTEGER PRIMARY KEY AUTOINCREMENT,
"species" TEXT NOT NULL,
"age" TEXT NOT NULL,
"ring" TEXT,
"animaltype_id" char(36) NOT NULL,
"discovery_id" char(36) NOT NULL,
"intake_id" char(36) NOT NULL,
"outtake_id" char(36),
"created_at" DATETIME NOT NULL,
"updated_at" DATETIME NOT NULL,
FOREIGN KEY (animaltype_id) REFERENCES animaltypes (id),
FOREIGN KEY (discovery_id) REFERENCES discovery (id),
FOREIGN KEY (intake_id) REFERENCES intakes (id),
FOREIGN KEY (outtake_id) REFERENCES outtakes (id)
);
CREATE TABLE sqlite_sequence(name,seq);
CREATE UNIQUE INDEX "users_email_idx" ON "users" (email);
CREATE UNIQUE INDEX "animaltypes_name_idx" ON "animaltypes" (name);
CREATE UNIQUE INDEX "outtaketypes_name_idx" ON "outtaketypes" (name);
