# Creaves Action Handlers Documentation

## Core Infrastructure

### app.go
**File**: `actions/app.go`

**Routes & Handlers**:
| Method | Route | Handler | Description |
|--------|-------|---------|-------------|
| GET | `/paths` | PathHandler | Serves paths page |
| GET | `/` | LandingIndex | Default landing page |
| GET | `/auth/` | AuthLanding | Auth landing page |
| GET | `/auth/new` | AuthNew | Login form |
| POST | `/auth/` | AuthCreate | Login submission |
| DELETE | `/auth/` | AuthDestroy | Logout |
| GET | `/lang/` | SwitchLanguage | Language switch (GET) |
| POST | `/lang/` | SwitchLanguagePost | Language switch (POST) |
| GET | `/registration/new` | UsersNew | Registration form |
| POST | `/registration/` | UsersCreate | Registration submission |
| GET | `/reception/new` | ReceptionNew | Reception form |
| GET | `/guest/` | GuestNew | Public guest status form / direct QR link (no auth) |
| POST | `/guest/` | GuestCreate | Guest status lookup (phone-verified, no auth) |
| GET | `/landing/index` | LandingIndex | Landing page (explicit) |
| GET | `/dashboard` | DashboardIndex | Dashboard view |
| GET | `/registertable` | RegistertableIndex | Register table |
| GET | `/registertable/ExportCSV` | RegistertableIndexCSV | Register CSV export |
| GET | `/registersnapshot` | RegistersnapshotIndex | Register snapshot |
| GET | `/registersnapshot/ExportCSV` | RegistersnapshotIndexCSV | Snapshot CSV export |
| GET | `/maintenance/` | MaintenanceIndex | Maintenance page |
| GET | `/maintenance/renumber` | MaintenanceRenumber | Renumber animals |
| GET | `/export/csv` | ExportCsv | CSV download (redirects to `/export/view` without `?query=`) |
| GET | `/export/view` | ExportView | Online export chooser / HTML table view |
| GET | `/export/excel` | ExportExcel | Excel export |
| GET | `/feeding` | FeedingIndex | Feeding schedule |
| GET | `/feeding/close` | FeedingClose | Close feeding |
| GET | `/crash` | Anonymous func | Intentional crash for testing |
| PUT | `/treatmentschedule` | TreatmentUpdateSchedule | Update treatment schedule (AJAX) |

**AJAX Endpoints**:
- `/suggestions/animal_species` - Species suggestions
- `/suggestions/discovery_location` - Discovery location suggestions
- `/suggestions/outtake_location` - Outtake location suggestions
- `/suggestions/discoverer_city` - City suggestions
- `/suggestions/discoverer_country` - Country suggestions
- `/suggestions/animal_in_care` - In-care animal suggestions
- `/suggestions/treatment_drug` - Drug suggestions
- `/suggestions/treatment_drug_dosage` - Drug dosage suggestions
- `/suggestions/CageWithAnimalInCare` - Cage suggestions
- `/suggestions/animaltype_species` - Animal type default species
- `/suggestions/postal_code` - Postal code suggestions
- `/suggestions/locality` - Locality suggestions
- `/suggestions/discoverer` - Discoverer suggestions
- `/hint/speciesDetails` - Species details hint

**Middleware**:
- `paramlogger.ParameterLogger` - Request parameter logging
- `csrf.New` - CSRF protection
- `popmw.Transaction(models.DB)` - DB transaction wrapping
- `translations()` - I18n middleware
- `SetCurrentUser` - Load current user from session
- `Authorize` - Require authentication

**Key Functions**:
- `App()` - Application bootstrap and route registration
- `translations()` - Setup i18n translator
- `forceSSL()` - SSL redirect middleware (disabled)

---

### auth.go
**File**: `actions/auth.go`

**Handlers**:
| Method | Route | Handler | Description |
|--------|-------|---------|-------------|
| GET | `/auth/` | AuthLanding | Shows login landing page |
| GET | `/auth/new` | AuthNew | Renders login form |
| POST | `/auth/` | AuthCreate | Authenticates user, sets session |
| DELETE | `/auth/` | AuthDestroy | Clears session, logs out |

**Business Logic**:
- `AuthCreate`: Validates login/password with bcrypt, checks `Approved` status, sets `current_user_id` in session, redirects to stored URL or home
- `AuthDestroy`: Clears session, flashes logout message

**Middleware**: Skips `Authorize` for auth routes

---

### users.go
**File**: `actions/users.go`

**Resource**: `UsersResource` (buffalo.Resource)

**Handlers**:
| Method | Route | Handler | Auth |
|--------|-------|---------|------|
| GET | `/registration/new` | UsersNew | Public |
| POST | `/registration/` | UsersCreate | Public |
| GET | `/users` | UsersResource.List | Admin only |
| GET | `/users/new` | UsersResource.New | Admin only |
| POST | `/users` | UsersResource.Create | Admin only |
| GET | `/users/{id}` | UsersResource.Show | Admin or self |
| GET | `/users/{id}/edit` | UsersResource.Edit | Admin or self (not shared) |
| PUT | `/users/{id}` | UsersResource.Update | Admin or self (not shared) |
| DELETE | `/users/{id}` | UsersResource.Destroy | Admin or self (not shared) |

**Middleware Functions**:
- `SetCurrentUser(next)` - Loads user from session into context
- `Authorize(next)` - Redirects to login if not authenticated
- `GetCurrentUser(c)` - Retrieves user from context

**Business Logic**:
- Registration creates unapproved accounts
- Admin-only for list/create
- Users can edit own profile (unless `Shared`)
- Cannot delete "admin" user

---

### render.go
**File**: `actions/render.go`

**Purpose**: Template engine initialization

**Key Features**:
- HTML layout: `application.plush.html`
- Template helpers:
  - `bool2html` - Converts bool to checkmark/cross
  - `dbgDump` - Debug output

---

### helper.go
**File**: `actions/helper.go`

**Utility Functions**:
- `sha256(s string)` - SHA1 hash (note: misnamed, uses SHA1)
- `timeToNullTime(s string)` - Parse "15:04" to nulls.Time
- `timeToMinutes(s string)` - Parse "15:04" to minutes
- `switchTimeZone(t, l)` - Change timezone of time

---

## Animal Lifecycle

### animals.go
**File**: `actions/animals.go`

**Resource**: `AnimalsResource`

**Handlers**:
| Method | Route | Handler | Description |
|--------|-------|---------|-------------|
| GET | `/animals` | List | List all animals with pagination |
| GET | `/animals/{id}` | Show | Show animal details |
| GET | `/animals/new` | New | New animal form |
| POST | `/animals` | Create | Create animal(s) |
| GET | `/animals/{id}/edit` | Edit | Edit animal form |
| PUT | `/animals/{id}` | Update | Update animal |
| DELETE | `/animals/{id}` | Destroy | "Delete" animal (creates error outtake) |

**Key Functions**:
- `EnrichAnimalsOptimized(a, c)` - Bulk query optimization for animal lists
- `EnrichAnimals(a, c)` - Original enrichment (legacy)
- `EnrichAnimal(a, c)` - Full enrichment for single animal
- `setupContext(c)` - Loads reference data for forms

**Business Logic**:
- Create supports batch creation via `AnimalCount` parameter
- Auto-generates `Year`/`YearNumber` from max existing
- Update creates care records on cage move or feeding change
- Destroy creates an "error" outtake (admin only)
- Supports lookup by `animal_year_number` parameter (e.g., "123/23")

---

### discoveries.go
**File**: `actions/discoveries.go`

**Resource**: `DiscoveriesResource`

**Handlers**:
| Method | Route | Handler | Auth |
|--------|-------|---------|------|
| GET | `/discoveries` | List | All |
| GET | `/discoveries/{id}` | Show | All |
| GET | `/discoveries/new` | New | All |
| POST | `/discoveries` | Create | All |
| GET | `/discoveries/{id}/edit` | Edit | All |
| PUT | `/discoveries/{id}` | Update | All |
| DELETE | `/discoveries/{id}` | Destroy | Admin only |

**Business Logic**:
- Destroy also destroys associated discoverer

---

### discoverers.go
**File**: `actions/discoverers.go`

**Resource**: `DiscoverersResource`

**Handlers**: Standard CRUD (List, Show, New, Create, Edit, Update, Destroy)

**Key Functions**:
- `FindByID(c, id)` - Helper to find discoverer by ID

**Auth**: Destroy admin-only

---

### intakes.go
**File**: `actions/intakes.go`

**Resource**: `IntakesResource`

**Handlers**: Standard CRUD

**Auth**: Destroy admin-only

---

### outtakes.go
**File**: `actions/outtakes.go`

**Resource**: `OuttakesResource`

**Handlers**:
| Method | Route | Handler | Auth |
|--------|-------|---------|------|
| GET | `/outtakes` | List | All |
| GET | `/outtakes/{id}` | Show | All |
| GET | `/outtakes/new` | New | All |
| POST | `/outtakes` | Create | All |
| GET | `/outtakes/{id}/edit` | Edit | Admin only |
| PUT | `/outtakes/{id}` | Update | Admin only |
| DELETE | `/outtakes/{id}` | Destroy | Admin only |

**Business Logic**:
- Create links to animal via `animal_id`, updates animal's `OuttakeID`
- Deletes future treatments on outtake creation
- Supports `animal_year_number` parameter for pre-population
- Destroy unlinks animal (sets `OuttakeID` to null)

---

### cares.go
**File**: `actions/cares.go`

**Resource**: `CaresResource`

**Handlers**:
| Method | Route | Handler | Auth |
|--------|-------|---------|------|
| GET | `/cares` | List | All |
| GET | `/cares/{id}` | Show | All |
| GET | `/cares/new` | New | All |
| POST | `/cares` | Create | All |
| GET | `/cares/{id}/edit` | Edit | All |
| PUT | `/cares/{id}` | Update | All |
| DELETE | `/cares/{id}` | Destroy | Admin only |

**Key Functions**:
- `EnrichCares(cs, c)` - Bulk enrichment for care lists
- `EnrichCaresWithAnimalNumber(cs, c)` - Enrichment with animal number
- `loadAnimalByCage(cage, c)` - Find animals by cage

**Business Logic**:
- Create supports bulk creation via `cage` parameter (creates care for all animals in cage)
- Invalidates weight loss cache on weight changes
- Supports `animal_year_number`, `cage`, `note`, `careType` params for pre-population

---

### treatments.go
**File**: `actions/treatments.go`

**Resource**: `TreatmentsResource`

**Handlers**:
| Method | Route | Handler | Auth |
|--------|-------|---------|------|
| GET | `/treatments` | List | All |
| GET | `/treatments/{id}` | Show | All |
| GET | `/treatments/new` | New | All |
| POST | `/treatments` | Create | All |
| GET | `/treatments/{id}/edit` | Edit | All |
| PUT | `/treatments/{id}` | Update | All |
| DELETE | `/treatments/{id}` | Destroy | All |
| PUT | `/treatmentschedule` | TreatmentUpdateSchedule | All (AJAX) |

**AJAX Endpoint**:
- `TreatmentUpdateSchedule` - Updates treatment `Timedonebitmap` via JSON

**Business Logic**:
- Create supports multi-date creation via comma-separated dates
- Uses `TreatmentTemplate` for batch creation
- Schedule encoding via `treatmentSchedule` struct

---

### veterinaryvisits.go
**File**: `actions/veterinaryvisits.go`

**Resource**: `VeterinaryvisitsResource`

**Handlers**: Standard CRUD

**Key Functions**:
- `setContext(c)` - Loads users for select dropdown

**Auth**: Destroy admin-only

---

### travels.go
**File**: `actions/travels.go`

**Resource**: `TravelsResource`

**Handlers**: Standard CRUD

**Auth**: 
- List: shows all for admin, own for non-admin
- Show/Edit/Update/Destroy: admin or owner only

**Business Logic**:
- Pre-populates with current user
- Supports `animal_year_number` parameter

---

## Audit Logging

### animal_audit.go
**File**: `actions/animal_audit.go`

Best-effort audit trail for every animal-related mutation. Handlers call
`auditAnimalChange(c, tx, animalID, entity, entityID, action, oldRec, newRec)`
after a successful persistence operation; a JSON-based diff (`models.ComputeChanges`)
produces the human readable change summary stored in `animal_audits`.

**Covered handlers** (create / update / delete):
- `animals.go` — animal create/update/destroy (incl. auto-created cage-move
  and feeding cares, error outtake on destroy)
- `cares.go` — cares (incl. cage mode multi-animal creation)
- `treatments.go` — treatments (incl. `TreatmentUpdateSchedule` status flips
  and post-outtake purge of scheduled treatments)
- `veterinaryvisits.go`, `travels.go`
- `intakes.go`, `outtakes.go`, `discoveries.go` (animal resolved via
  `animals.intake_id` / `animals.outtake_id` / `animals.discovery_id`;
  entries skipped when unlinked)

**Display**: admin-only "Audit" tab on the animal show page
(`templates/animals/show.plush.html` + localized variants), server-side
pagination via `PaginateFromParams` (`?page=` / `?per_page=`), newest first.

**Guarantees**:
- Logging failures never abort the business operation (logged via `c.Logger()`).
- Updates without a detectable diff are not recorded.
- `user_name` is denormalized so entries stay readable after user deletion.

---

## Reference Data

All reference data resources follow standard Buffalo CRUD patterns:

### animaltypes.go
**Admin-only**: List, Show, New, Create, Edit, Update, Destroy
- Resets `Default` and `HasRing` flags on update

### animalages.go
**Admin-only**: All operations

### caretypes.go
**Open**: List, Show, New, Create, Edit, Update, Destroy (no admin restriction)

### outtaketypes.go
**Admin-only**: List, Show, New, Create, Edit, Update, Destroy
- Resets `Default` and `Dead` flags on update

### traveltypes.go
**Admin-only**: New, Create, Edit, Update, Destroy
**Open**: List, Show

### drugs.go
**Admin-only**: All operations

**Key Functions**:
- `enrichDrug(drug, c)` - Loads dosages with animal types
- `saveDrugs(d, c)` - Saves drug and associated dosages
- `setContext(c)` - Loads animal types

**Business Logic**:
- Dosages stored per animal type
- Converts dosage from per-kg to per-gram on save

### species.go
**Admin-only**: All operations
- Resets `Game` flag on update
- Ordered by class, order, family, species

### zones.go
**Open**: All operations
- Ordered by zone name

### localities.go
**Open**: All operations
- Ordered by Country, Region, Province, Municipality

### entry_causes.go
**Open**: All operations
- Ordered by sort_order

### native_statuses.go
**Open**: All operations

### subside_groups.go
**Open**: All operations

---

## Pages

### landing.go
**File**: `actions/landing.go`

**Handler**: `LandingIndex` (GET `/`, `/landing/index`)

**Business Logic**:
- Loads all animals with `outtake_id IS NULL`
- Groups by type and zone
- Identifies animals with clean cage in last 24h
- Uses `EnrichAnimalsOptimized`

**Key Functions**:
- `listAnimalWithCleanCage(c)` - SQL query for recent clean cages

---

### guest.go
**File**: `actions/guest.go`

**Handlers**:
| Method | Route | Handler | Description |
|--------|-------|---------|-------------|
| GET | `/guest/` | GuestNew | Public form: animal number + discoverer phone; direct QR link with `number` + `token` + `lang` |
| POST | `/guest/` | GuestCreate | Verification + succinct status view |

Public (unauthenticated) status page for animal discoverers. Routes are declared in an
`/guest` group with `Middleware.Remove(Authorize)` (same pattern as `/registration`).
Guest pages render inside the minimal `guest.plush.html` layout: the only menu is the
language selector.

**Direct link (QR code)**: the QR code on the animal page encodes
`<scheme>://<host>/guest/?number=<num>&token=<token>&lang=<lang>`. Scheme and host are
taken from the incoming request (`Request.Host`, `X-Forwarded-Proto` honoured), so the
QR always points at the same host/protocol the staff page was loaded from. The token is
`guestPhoneToken` — a SHA-256 hash of the normalized discoverer phone, salted with the
animal number — so the raw phone never appears in the URL and scanning the code opens
the status view directly (no form). `lang` is applied before rendering (cookie +
`T.Refresh`) so the page opens in the language the QR was generated with (current UI
language, default French).
A failed token/number falls back to the plain form without disclosing the reason; the
plain form link (full URL, same request-derived scheme/host — `guestFormURL` set by
`AnimalsResource.Show`) is displayed under the QR code in the modal.

**Business Logic**:
- Lookup by animal number `"123"` or `"123/24"` (same grammar as the animals list;
  latest animal wins when the year is omitted).
- Phone verification via `guestPhoneMatches`: digits-only comparison with a suffix
  match (>= `guestPhoneMinSuffix` digits, both directions) plus a last-9-digits
  comparison for national vs international notation (`0612345678` <-> `+33612345678`).
- Mismatch (or unknown animal/number) re-renders the form with a generic error —
  no information is disclosed about whether the animal number exists.
- Per-IP in-memory rate limiting: `guestRateMax` attempts per `guestRateWindow`
  (30 / 15 min); excess returns HTTP 429. Single-instance only (resets on restart).

**Guest view content** (`guestView`):
- Always: animal number, species (localized via `tspecies`), arrival date
  (intake date, fallback discovery date).
- Animal still in care (`outtake_id IS NULL`): care-intensity status from
  `decideGuestCareStatus`:
  - *critical*: force-fed, next planned feeding overdue (feeding code 0, i.e. more
    than half a period late), or a planned treatment slot for today whose cutoff
    passed unmarked (slot cutoffs: morning 12:00, noon 17:00, evening 23:59 —
    `guestSlotMissed`);
  - *intensive*: at least one treatment planned today;
  - *care*: otherwise.
  Also shows the next planned feeding time.
- Animal has left: outtake date plus the "News for Discoverer" text
  (`outtaketypes.discoverer_news`) of the outtake type, falling back to nothing
  when unset.

**Multilingual**: template variants `templates/guest/new.plush.{,fr,de,nl}.html` and
`templates/guest/show.plush.{,fr,de,nl}.html`, selected by request language.

**Key Functions**:
- `guestFindAnimal(tx, number)` - year-number lookup, `(nil, nil)` when unknown
- `guestPhoneMatches(stored, given)` - phone verification
- `guestCareInfo(tx, animal, now)` - care status + next feeding
- `guestRateAllow(key, now)` - sliding-window rate limiter
- `buildGuestView(tx, animal, now)` - view assembly

**Tests**: `actions/guest_test.go` (pure logic + DB integration, skipped when the
test database is unavailable).

---

### dashboard.go
**File**: `actions/dashboard.go`

**Handler**: `DashboardIndex` (GET `/dashboard`)

**Business Logic**:
- Shows animals with weight loss (>7% decrease)
- Shows open cares (warning status)
- Shows animals with today's treatments
- Shows force-feed animals
- Shows animal count per type
- Shows last 24h log entries

**Key Functions**:
- `listOpenCares(c)` - Cares needing attention
- `listAnimalWithTodayTreatments(c)` - Today's treatments
- `listAnimalWithForceFeed(c)` - Force feed list
- `listAnimalCountPerType(c)` - Count statistics
- `listLast24hLogEntries(c)` - Recent activity

---

### feeding.go
**File**: `actions/feeding.go`

**Handlers**:
| Method | Route | Handler |
|--------|-------|---------|
| GET | `/feeding` | FeedingIndex |
| GET | `/feeding/close` | FeedingClose |

**Business Logic**:
- Calculates next feeding times based on schedule
- Groups by zone
- Color codes: late (0), in-time (1), near (2), future (3)
- Close creates a care record

**Key Functions**:
- `calculateFeeding(af, now)` - Calculate next feeding
- `calculateFeedings(afRaw)` - Batch calculation
- `generateAnimalFeedings(c)` - Load and calculate

---

### reception.go
**File**: `actions/reception.go`

**Handler**: `ReceptionNew` (GET `/reception/new`)

**Business Logic**:
- Pre-populated reception form
- Sets default country (Belgique)
- Sets default zone
- Sets default animal type

---

### registertable.go
**File**: `actions/registertable.go`

**Handlers**:
| Method | Route | Handler |
|--------|-------|---------|
| GET | `/registertable` | RegistertableIndex |
| GET | `/registertable/ExportCSV` | RegistertableIndexCSV |

**Business Logic**:
- Shows animals by year
- CSV export support

---

### registersnapshot.go
**File**: `actions/registersnapshot.go`

**Handlers**:
| Method | Route | Handler |
|--------|-------|---------|
| GET | `/registersnapshot` | RegistersnapshotIndex |
| GET | `/registersnapshot/ExportCSV` | RegistersnapshotIndexCSV |

**Business Logic**:
- Shows animals in care on a specific date
- Defaults to today
- CSV export support

---

### export.go
**File**: `actions/export.go`

**Handlers**:
| Method | Route | Handler |
|--------|-------|---------|
| GET | `/export/csv` | ExportCsv |
| GET | `/export/view` | ExportView |
| GET | `/export/excel` | ExportExcel |

**Business Logic**:
- `/export/csv` without a query param redirects to `/export/view`
- `/export/csv?query=...` executes the query via the `export` package and downloads a CSV
- `/export/view` without a query renders the chooser; with a query renders an HTML table

---

### maintenance.go
**File**: `actions/maintenance.go`

**Handlers**:
| Method | Route | Handler | Auth |
|--------|-------|---------|------|
| GET | `/maintenance/` | MaintenanceIndex | Admin |
| GET | `/maintenance/renumber` | MaintenanceRenumber | Admin |

**Business Logic**:
- Renumber: Resets and recalculates `year` and `yearNumber` for all animals

---

## Utilities

### typehelper.go
**File**: `actions/typehelper.go`

**Purpose**: Load reference data and convert to form selectables

**Functions**:
- `animalTypes(c)` - Load animal types
- `animalTypesToSelectables(ts)` - Convert to select options
- `zones(c)` / `zonesMap(c)` / `zonesToSelectables(ts)` - Zone helpers
- `defZone(c)` - Get default zone
- `outtakeTypes(c)` / `outtakeTypesToSelectables(ts)` - Outtake type helpers
- `caretypes(c)` / `caretypesToSelectables(ts)` - Care type helpers
- `traveltypes(c)` / `traveltypesToSelectables(ts)` - Travel type helpers
- `animalages(c)` / `animalagesToSelectables(ts)` - Age helpers
- `users(c)` / `usersToMap(us)` / `usersToSelectables(ts)` - User helpers
- `entryCauses(c)` / `entryCausesToSelectables(ts, withBlank)` - Entry cause helpers
- `selectFeedingPeriod()` - Feeding period options
- `BoolToInt(b)` - Bool to int conversion
- `AnimalYearNumberRegEx` - Regex for year/number parsing

---

### cache_utils.go
**File**: `actions/cache_utils.go`

**Purpose**: Weight loss data caching

**Functions**:
- `GetWeightLossData(c)` - Get cached or fresh weight loss data
- `InvalidateWeightLossCache()` - Mark cache stale
- `refreshWeightLossCache()` - Background refresh (stub)

**Cache**: 12-hour TTL, invalidated on care weight changes

---

### suggestions.go
**File**: `actions/suggestions.go`

**Purpose**: AJAX autocomplete endpoints

**Handlers** (all GET, return JSON):
- `SuggestionsAnimalSpecies` - Species names
- `SuggestionsDiscoveryLocation` - Discovery locations
- `SuggestionsOuttakeLocation` - Outtake locations (postal_locality format)
- `SuggestionsDiscovererCity` - Discoverer cities
- `SuggestionsDiscovererCountry` - Discoverer countries
- `SuggestionsPostalCode` - Postal codes
- `SuggestionsLocality` - Localities (filtered by zip/locality)
- `SuggestionsDiscoverer` - Discoverers (filtered by name/address)
- `SuggestionsAnimalTypeDefaultSpecies` - Default species per type
- `SuggestionsTreatmentDrug` - Drugs by animal type
- `SuggestionsTreatmentDrugDosage` - Calculate dosage by weight
- `SuggestionsAnimalInCare` - In-care animals (yearNumber/year format)
- `SuggestionsCageWithAnimalInCare` - Cages with animals

---

### hint.go
**File**: `actions/hint.go`

**Handler**: `HintSpeciesDetails` (GET `/hint/speciesDetails`)

**Purpose**: Returns species details (native status, freeable, game, huntable) as JSON

---

### switchLanguage.go
**File**: `actions/switchLanguage.go`

**Handlers**:
| Method | Route | Handler |
|--------|-------|---------|
| GET | `/lang/` | SwitchLanguage |
| POST | `/lang/` | SwitchLanguagePost |

**Business Logic**:
- Sets `lang` cookie (1 year expiry)
- Refreshes translator
- Redirects back to original URL

---

### paths.go
**File**: `actions/paths.go`

**Handler**: `PathHandler` (GET `/paths`)

**Purpose**: Serves paths/index.html

---

### logentries.go
**File**: `actions/logentries.go`

**Resource**: `LogentriesResource`

**Handlers**: Standard CRUD

**Auth**: 
- Edit/Update/Destroy: Admin or owner only
- Create: Sets current user automatically

---

### dashboard_weightloss.go
**File**: `actions/dashboard_weightloss.go`

**Purpose**: Weight loss detection for dashboard

**Functions**:
- `listAnimalWithWeightLoss(c)` - Finds animals with >7% weight loss in 10 days

**SQL**: Complex CTE query comparing oldest and newest weights

---

## Middleware Summary

| Middleware | File | Description |
|------------|------|-------------|
| `SetCurrentUser` | users.go | Loads user from session ID |
| `Authorize` | users.go | Requires authentication |
| `ParameterLogger` | app.go | Logs request params |
| `csrf.New` | app.go | CSRF token protection |
| `popmw.Transaction` | app.go | DB transaction per request |
| `translations` | app.go | I18n locale resolution |

## Common Patterns

1. **Resource Structure**: All CRUD resources embed `buffalo.Resource`
2. **Responder Pattern**: Most handlers support HTML/JSON/XML via `responder.Wants`
3. **Auth Checks**: Admin checks use `GetCurrentUser(c).Admin`
4. **Transaction Access**: `tx := c.Value("tx").(*pop.Connection)`
5. **Enrichment**: Two patterns - `EnrichAnimals` (legacy) and `EnrichAnimalsOptimized` (bulk queries)
6. **Year/Number Lookup**: `AnimalYearNumberRegEx` parses formats like "123/23"
7. **Flash Messages**: Success/error messages via `c.Flash().Add()`
8. **AJAX**: Suggestions and hints return JSON; treatment schedule uses PUT with JSON
