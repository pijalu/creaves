# Admin browser review bugs

- [x] Species New/Edit only loaded and saved `creaves_species` translations while startup data defines seven translatable fields. Fixed controller field lists and verified `/species/new/` now exposes 24 translation inputs.
- [x] Full per-language edit mutation sweep completed for animalages, animaltypes, caretypes, drugs, outtaketypes, zones, and traveltypes using authenticated agent-browser clicks; each submitted successfully and redirected to show pages. QA values remain in local test database for audit.
- [x] Initial browser daemon session lost authentication and earlier click tests appeared as GET/login redirects. Fresh agent-browser session authenticated admin; accessibility click now correctly sends PUT and redirects 303. Verified with `juvenile CLICK`.