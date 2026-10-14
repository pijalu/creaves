-- needs_confirmation: manual "Need confirmation" flag (see BUG-52e user ruling).
-- Converter never sets it automatically; users/admins set/unset it via UI
-- to surface a plan on the "Protocols to confirm" screen.
ALTER TABLE care_animal_plans ADD COLUMN needs_confirmation TINYINT(1) NOT NULL DEFAULT 0;
