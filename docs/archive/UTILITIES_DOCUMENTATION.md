# Creaves Project - Utility Functions and Helpers Documentation

## Table of Contents
1. [actions/helper.go](#actionshelpergo)
2. [actions/typehelper.go](#actionstypehelpergo)
3. [actions/cache_utils.go](#actionscache_utilsg)
4. [utils/utils.go](#utilsutilsg)
5. [export/export.go](#exportexportgo)
6. [localrender/csv.go](#localrendercsvg)
7. [actions/render.go](#actionsrendergo)
8. [actions/animals.go - Enrichment Functions](#actionsanimalsgo---enrichment-functions)
9. [actions/cares.go - Enrichment Functions](#actionscaresgo---enrichment-functions)
10. [actions/users.go - Auth Helpers](#actionsusersgo---auth-helpers)
11. [actions/suggestions.go](#actionssuggestionsgo)
12. [actions/feeding.go](#actionsfeedinggo)
13. [actions/dashboard.go](#actionsdashboardgo)
14. [actions/landing.go](#actionslandinggo)
15. [actions/registertable.go](#actionsregistertablego)
16. [actions/registersnapshot.go](#actionsregistersnapshotgo)
17. [actions/reception.go](#actionsreceptiongo)
18. [actions/maintenance.go](#actionsmaintenancego)
19. [actions/export.go](#actionsexportgo)
20. [actions/hint.go](#actionshintgo)
21. [actions/switchLanguage.go](#actionsswitchlanguagego)
22. [actions/paths.go](#actionspathsgo)
23. [actions/dashboard_weightloss.go](#actionsdashboard_weightlossgo)
24. [actions/app.go](#actionsappgo)

---

## actions/helper.go

**File Path**: `actions/helper.go`

General helper functions for string hashing, time conversion, and timezone manipulation.

### Functions

#### `sha256(s string) string`
- **Signature**: `func sha256(s string) string`
- **Purpose**: Generates a SHA1 hash of a string (despite the name suggesting SHA256)
- **Parameters**:
  - `s`: Input string to hash
- **Returns**: Hex-encoded hash string
- **Note**: Function name is misleading - it uses SHA1, not SHA256

#### `timeToNullTime(s string) nulls.Time`
- **Signature**: `func timeToNullTime(s string) nulls.Time`
- **Purpose**: Parses a time string in "15:04" format and converts it to a `nulls.Time`
- **Parameters**:
  - `s`: Time string in "HH:MM" format
- **Returns**: `nulls.Time` with parsed time or empty `nulls.Time{}` if parsing fails
- **Behavior**: Sets date to 0001-01-01 and timezone to UTC

#### `timeToMinutes(s string) int`
- **Signature**: `func timeToMinutes(s string) int`
- **Purpose**: Converts a time string to total minutes since midnight
- **Parameters**:
  - `s`: Time string in "HH:MM" format
- **Returns**: Total minutes (e.g., "01:30" returns 90) or 0 if parsing fails

#### `switchTimeZone(t time.Time, l *time.Location) time.Time`
- **Signature**: `func switchTimeZone(t time.Time, l *time.Location) time.Time`
- **Purpose**: Changes the timezone of a time value while preserving the clock time
- **Parameters**:
  - `t`: Source time
  - `l`: Target location/timezone
- **Returns**: New time.Time in the specified location

---

## actions/typehelper.go

**File Path**: `actions/typehelper.go`

Type loading helpers that fetch reference data from the database and convert it to form selectables. All functions require a `buffalo.Context` with a transaction.

### Types

#### `selType`
```go
type selType struct {
    label string
    value interface{}
}
```
- Implements `form.Selectable` interface
- Methods:
  - `SelectValue() interface{}` - Returns the value
  - `SelectLabel() string` - Returns the label

### Functions

#### `animalTypes(c buffalo.Context) (*models.Animaltypes, error)`
- Fetches all animal types ordered by name ascending
- Returns pointer to `models.Animaltypes` collection

#### `animalTypesToSelectables(ts *models.Animaltypes) form.Selectables`
- Converts animal types to form selectables
- Adds empty option at start
- Removes empty option if any type has `Default` flag set

#### `defZone(c buffalo.Context) (*models.Zone, error)`
- Fetches the default zone (where `default` is true)
- Orders by zone name ascending

#### `zones(c buffalo.Context) (*models.Zones, error)`
- Fetches all zones ordered by zone name ascending

#### `zonesMap(c buffalo.Context) (map[string]string, error)`
- Converts zones to a map: `zone_name -> zone_type`

#### `zonesToSelectables(ts *models.Zones) form.Selectables`
- Converts zones to form selectables
- Uses zone name as both label and value

#### `outtakeTypes(c buffalo.Context) (*models.Outtaketypes, error)`
- Fetches all outtake types ordered by name ascending

#### `outtakeTypesToSelectables(ts *models.Outtaketypes) form.Selectables`
- Converts outtake types to form selectables

#### `caretypes(c buffalo.Context) (*models.Caretypes, error)`
- Fetches all care types ordered by name ascending

#### `caretypesToSelectables(ts *models.Caretypes) form.Selectables`
- Converts care types to form selectables

#### `traveltypes(c buffalo.Context) (*models.Traveltypes, error)`
- Fetches all travel types ordered by name ascending

#### `traveltypesToSelectables(ts *models.Traveltypes) form.Selectables`
- Converts travel types to form selectables

#### `animalages(c buffalo.Context) (*models.Animalages, error)`
- Fetches all animal ages ordered by name ascending

#### `animalagesToSelectables(ts *models.Animalages) form.Selectables`
- Converts animal ages to form selectables

#### `users(c buffalo.Context) (*models.Users, error)`
- Fetches all users ordered by login ascending

#### `usersToMap(us *models.Users) map[uuid.UUID]models.User`
- Converts users slice to map keyed by UUID

#### `usersToSelectables(ts *models.Users) form.Selectables`
- Converts users to form selectables (uses login as label)

#### `selectFeedingPeriod() form.Selectables`
- Returns predefined feeding period options
- Options: N.A. (0), 15min (15), 30min (30), 1h-12h (60-720)

#### `BoolToInt(b bool) int`
- Converts boolean to integer (true=1, false=0)

#### `entryCauses(c buffalo.Context) (*models.EntryCauses, error)`
- Fetches all entry causes ordered by sort_order ascending

#### `entryCausesToSelectables(ts *models.EntryCauses, withBlank bool) form.Selectables`
- Converts entry causes to form selectables
- Optionally adds blank option at start
- Uses `Fmt(true)` for formatted label

### Global Variables

#### `AnimalYearNumberRegEx`
```go
var AnimalYearNumberRegEx = regexp.MustCompile(`(\d+)(/(\d{2}))?`)
```
- Regular expression for parsing animal year numbers
- Matches patterns like "123" or "123/24"
- Groups: (1) number, (3) optional 2-digit year suffix

---

## actions/cache_utils.go

**File Path**: `actions/cache_utils.go`

Caching utilities for weight loss data with thread-safe operations.

### Global Variables

```go
var (
    weightLossCache     *[]AnimalWithWeight
    cacheMutex          sync.RWMutex
    cacheLastUpdate     time.Time
    cacheUpdateInterval = 12 * time.Hour
)
```

### Functions

#### `init()`
- Initializes a background goroutine that refreshes cache every 12 hours
- Note: `refreshWeightLossCache()` is currently a no-op (needs Buffalo context for DB access)

#### `GetWeightLossData(c buffalo.Context) (*[]AnimalWithWeight, error)`
- **Signature**: `func GetWeightLossData(c buffalo.Context) (*[]AnimalWithWeight, error)`
- **Purpose**: Returns cached weight loss data or fetches fresh data if cache is stale
- **Parameters**:
  - `c`: Buffalo context with database transaction
- **Returns**: Pointer to slice of `AnimalWithWeight` or error
- **Thread Safety**: Uses RWMutex for concurrent access
- **Cache Logic**:
  1. Checks if cache exists and is within 12-hour window
  2. Returns cached copy if valid
  3. Otherwise fetches fresh data via `listAnimalWithWeightLoss(c)`
  4. Updates cache with new data

#### `InvalidateWeightLossCache()`
- **Signature**: `func InvalidateWeightLossCache()`
- **Purpose**: Marks cache as stale, forcing refresh on next access
- **Thread Safety**: Uses write lock
- **Behavior**: Sets `cacheLastUpdate` to zero time

---

## utils/utils.go

**File Path**: `utils/utils.go`

String utility functions using reflection.

### Functions

#### `TrimStringFields(s interface{})`
- **Signature**: `func TrimStringFields(s interface{})`
- **Purpose**: Trims trailing spaces from all string and `nulls.String` fields in a struct
- **Parameters**:
  - `s`: Pointer to a struct (must be pointer, not value)
- **Behavior**:
  - Iterates through all struct fields using reflection
  - For `string` fields: trims right spaces using `strings.TrimRight`
  - For `nulls.String` fields: trims only if `Valid` is true
  - Prints error message if input is not a pointer to struct
- **Example**:
  ```go
  type MyStruct struct {
      Name  string
      Email nulls.String
  }
  s := &MyStruct{Name: "John ", Email: nulls.NewString("john@example.com ")}
  TrimStringFields(s) // Trims trailing spaces
  ```

---

## export/export.go

**File Path**: `export/export.go`

CSV export functionality with YAML-configured SQL queries.

### Types

#### `Queries`
```go
type Queries struct {
    Name        string `yaml:"name"`
    Description string `yaml:"description"`
    Query       string `yaml:"query"`
}
```

#### `Config`
```go
type Config struct {
    Queries []Queries `yaml:"queries"`
}
```

### Functions

#### `getConfig() *Config`
- **Signature**: `func getConfig() *Config`
- **Purpose**: Loads export configuration from embedded YAML file
- **Behavior**: 
  - Reads `config.yaml` embedded at compile time
  - Panics if YAML parsing fails
  - Called once at package initialization

#### `(c *Config) getQuery(id string) (*Queries, error)`
- **Signature**: `func (c *Config) getQuery(id string) (*Queries, error)`
- **Purpose**: Retrieves a query by name (case-insensitive)
- **Parameters**:
  - `id`: Query name to look up
- **Returns**: Pointer to `Queries` or error if not found

#### `GetQueries() []Queries`
- **Signature**: `func GetQueries() []Queries`
- **Purpose**: Returns all configured queries
- **Returns**: Slice of all `Queries` from config

#### `RunQuery(c buffalo.Context, query string) error`
- **Signature**: `func RunQuery(c buffalo.Context, query string) error`
- **Purpose**: Executes a configured SQL query and streams results as CSV
- **Parameters**:
  - `c`: Buffalo context for response writing
  - `query`: Name of the query to execute
- **Behavior**:
  1. Opens direct SQL connection to database
  2. Looks up query by name
  3. Executes SQL query
  4. Writes results as CSV to HTTP response
  5. Sets headers: `Content-Type: text/csv`, `Content-Disposition: attachment`
- **Error Handling**: Returns 404 if query not found
- **Note**: Uses `sql.Open` directly, not the Pop transaction

---

## localrender/csv.go

**File Path**: `localrender/csv.go`

Custom CSV renderer for Buffalo that changes content type while delegating to plain text renderer.

### Types

#### `templateDelegatedRenderer`
```go
type templateDelegatedRenderer struct {
    delegate    render.Renderer
    contentType string
}
```

### Functions

#### `(t templateDelegatedRenderer) ContentType() string`
- Returns the configured content type ("text/csv; charset=utf-8")

#### `(s *templateDelegatedRenderer) Render(w io.Writer, data render.Data) error`
- Delegates rendering to the underlying renderer

#### `Csv(e *render.Engine, names ...string) render.Renderer`
- **Signature**: `func Csv(e *render.Engine, names ...string) render.Renderer`
- **Purpose**: Creates a CSV renderer that uses plain text templates but sets CSV content type
- **Parameters**:
  - `e`: Buffalo render engine
  - `names`: Template file names
- **Returns**: Renderer with "text/csv; charset=utf-8" content type
- **Usage**:
  ```go
  return c.Render(http.StatusOK, localrender.Csv(r, "mytemplate.plush.csv"))
  ```

---

## actions/render.go

**File Path**: `actions/render.go`

Buffalo render engine initialization with custom template helpers.

### Global Variables

#### `r *render.Engine`
- Global render engine instance

### Functions

#### `init()`
- Initializes the render engine with:
  - HTML layout: `application.plush.html`
  - Templates from embedded filesystem
  - Assets from embedded filesystem
  - Custom helpers:
    - `bool2html`: Converts bool to checkmark (✓) or cross (🞩)
    - `dbgDump`: Formats any value with `%v` for debugging

---

## actions/animals.go - Enrichment Functions

**File Path**: `actions/animals.go`

Functions for loading and enriching animal records with related data.

### Functions

#### `EnrichAnimalsOptimized(a *models.Animals, c buffalo.Context) (*models.Animals, error)`
- **Signature**: `func EnrichAnimalsOptimized(a *models.Animals, c buffalo.Context) (*models.Animals, error)`
- **Purpose**: Efficiently loads animal dependencies using bulk queries (N+1 optimized)
- **Parameters**:
  - `a`: Pointer to animals collection
  - `c`: Buffalo context with transaction
- **Behavior**:
  1. Collects unique IDs for: animal types, ages, discoveries, intakes, outtakes
  2. Bulk fetches each relation type in single queries
  3. Fetches today's treatments for all animals in one query
  4. Maps related data back to animals
- **Returns**: Enriched animals collection
- **Performance**: Much faster than `EnrichAnimals` for large collections

#### `containsUUID(slice []uuid.UUID, id uuid.UUID) bool`
- **Signature**: `func containsUUID(slice []uuid.UUID, id uuid.UUID) bool`
- **Purpose**: Checks if UUID slice contains a specific UUID
- **Helper for**: `EnrichAnimalsOptimized`

#### `EnrichAnimals(a *models.Animals, c buffalo.Context) (*models.Animals, error)`
- **Signature**: `func EnrichAnimals(a *models.Animals, c buffalo.Context) (*models.Animals, error)`
- **Purpose**: Original enrichment function (kept for compatibility)
- **Behavior**: Loads all reference data (types, ages, outtake types) then loads related records
- **Note**: Less efficient than `EnrichAnimalsOptimized` - loads all reference types even if not needed

#### `EnrichAnimal(a *models.Animal, c buffalo.Context) (*models.Animal, error)`
- **Signature**: `func EnrichAnimal(a *models.Animal, c buffalo.Context) (*models.Animal, error)`
- **Purpose**: Loads ALL dependencies for a single animal record
- **Loads**: Animalage, Animaltype, Discovery (with Discoverer), Intake, Cares, VetVisits, Treatments, Outtake
- **Use Case**: Detail/show pages where all related data is needed

#### `(v AnimalsResource) loadAnimal(animal_id string, c buffalo.Context) (*models.Animal, error)`
- **Signature**: `func (v AnimalsResource) loadAnimal(animal_id string, c buffalo.Context) (*models.Animal, error)`
- **Purpose**: Finds animal by ID and enriches it
- **Returns**: Fully enriched animal or 404 error

#### `setupContext(c buffalo.Context) error`
- **Signature**: `func setupContext(c buffalo.Context) error`
- **Purpose**: Populates context with form selectables for animal forms
- **Sets**:
  - `selectAnimalTypes`
  - `selectAnimalages`
  - `selectOuttaketype`
  - `selectFeedingPeriod`
  - `selectZone`
  - `selectEntryCause`

---

## actions/cares.go - Enrichment Functions

**File Path**: `actions/cares.go`

Functions for enriching care records.

### Functions

#### `EnrichCares(cs *models.Cares, c buffalo.Context) (*models.Cares, error)`
- **Signature**: `func EnrichCares(cs *models.Cares, c buffalo.Context) (*models.Cares, error)`
- **Purpose**: Loads care types and associated animals for care records
- **Behavior**:
  1. Loads all care types into map
  2. Associates care type with each care record
  3. Bulk loads animals by ID
  4. Associates animal with each care record

#### `EnrichCaresWithAnimalNumber(cs *[]models.CareWithAnimalNumber, c buffalo.Context) (*[]models.CareWithAnimalNumber, error)`
- **Signature**: `func EnrichCaresWithAnimalNumber(cs *[]models.CareWithAnimalNumber, c buffalo.Context) (*[]models.CareWithAnimalNumber, error)`
- **Purpose**: Similar to `EnrichCares` but for `CareWithAnimalNumber` type
- **Note**: Only loads care types, not animals (animal data already in struct)

#### `(v CaresResource) loadAnimalByCage(cage string, c buffalo.Context) (*[]models.Animal, error)`
- **Signature**: `func (v CaresResource) loadAnimalByCage(cage string, c buffalo.Context) (*[]models.Animal, error)`
- **Purpose**: Finds all animals in a specific cage that haven't been outtaken
- **Parameters**:
  - `cage`: Cage identifier
- **Returns**: Slice of animals or error

---

## actions/users.go - Auth Helpers

**File Path**: `actions/users.go`

Authentication and authorization helper functions.

### Functions

#### `SetCurrentUser(next buffalo.Handler) buffalo.Handler`
- **Signature**: `func SetCurrentUser(next buffalo.Handler) buffalo.Handler`
- **Purpose**: Middleware that loads current user from session
- **Behavior**:
  1. Checks session for `current_user_id`
  2. Loads user from database
  3. If user not found: clears session, sets redirect URL
  4. If user not approved: clears session, redirects to login
  5. Sets `current_user` on context
- **Middleware Order**: Applied after transaction middleware, before authorization

#### `Authorize(next buffalo.Handler) buffalo.Handler`
- **Signature**: `func Authorize(next buffalo.Handler) buffalo.Handler`
- **Purpose**: Middleware that requires authentication
- **Behavior**:
  1. Checks if `current_user_id` exists in session
  2. If not: saves redirect URL, flashes error, redirects to login
  3. If yes: continues to next handler
- **Skipped For**: Auth routes, language routes, registration routes

#### `GetCurrentUser(c buffalo.Context) *models.User`
- **Signature**: `func GetCurrentUser(c buffalo.Context) *models.User`
- **Purpose**: Retrieves current user from context
- **Returns**: User pointer or nil if not authenticated
- **Usage**: Used throughout handlers for permission checks

---

## actions/suggestions.go

**File Path**: `actions/suggestions.go`

Autocomplete/suggestion handlers for various form fields.

### Functions

#### `suggest(c buffalo.Context, table string, field string) error`
- **Signature**: `func suggest(c buffalo.Context, table string, field string) error`
- **Purpose**: Generic suggestion handler for distinct field values
- **Parameters**:
  - `table`: Database table name
  - `field`: Column name
- **Query Param**: `q` - optional search term (uses LIKE `%q%`)
- **Returns**: JSON array of matching strings

#### `SuggestionsAnimalSpecies(c buffalo.Context) error`
- Suggests species names from `species.creaves_species`

#### `SuggestionsDiscoveryLocation(c buffalo.Context) error`
- Suggests discovery locations from `discoveries.location`

#### `SuggestionsOuttakeLocation(c buffalo.Context) error`
- Suggests outtake locations as `postal_code_locality` from localities table

#### `SuggestionsDiscovererCity(c buffalo.Context) error`
- Suggests cities from `discoverers.city`

#### `SuggestionsDiscovererCountry(c buffalo.Context) error`
- Suggests countries from `discoverers.country`

#### `SuggestionsPostalCode(c buffalo.Context) error`
- Suggests postal codes from `localities.postal_code`

#### `SuggestionsLocality(c buffalo.Context) error`
- Suggests localities with filtering by zip and locality
- **Query Params**: `z` (zip), `l` (locality), `r` (return field)

#### `SuggestionsDiscoverer(c buffalo.Context) error`
- Suggests discoverer fields with filtering
- **Query Params**: `f` (firstname), `l` (lastname), `a` (address), `r` (return field)

#### `SuggestionsAnimalTypeDefaultSpecies(c buffalo.Context) error`
- Suggests default species from animal types

#### `SuggestionsTreatmentDrug(c buffalo.Context) error`
- Suggests drug names filtered by animal type
- **Query Params**: `q` (search), `at` (animal type ID)

#### `SuggestionsTreatmentDrugDosage(c buffalo.Context) error`
- Calculates and returns drug dosage based on weight
- **Query Params**: `q` (drug name), `at` (animal type ID), `w` (weight in grams)
- **Returns**: Formatted dosage string (e.g., "0.50 mg")

#### `SuggestionsAnimalInCare(c buffalo.Context) error`
- Suggests animals currently in care
- **Returns**: Formatted year numbers (e.g., "123/24")

#### `SuggestionsCageWithAnimalInCare(c buffalo.Context) error`
- Suggests cages that have animals in care

---

## actions/feeding.go

**File Path**: `actions/feeding.go`

Feeding schedule calculation and management.

### Types

#### `AnimalFeeding`
```go
type AnimalFeeding struct {
    ID              int          `db:"id"`
    Year            int          `db:"year"`
    YearNumber      int          `db:"yearNumber"`
    Species         string       `db:"species"`
    Cage            nulls.String `db:"cage"`
    Zone            nulls.String `db:"zone"`
    Feeding         string       `db:"feeding"`
    ForceFeed       bool         `db:"force_feed"`
    FeedingStart    time.Time    `db:"feeding_start"`
    FeedingEnd      time.Time    `db:"feeding_end"`
    FeedingPeriod   int          `db:"feeding_period"`
    LastFeeding     nulls.Time   `db:"last_feeding"`
    NextFeeding     nulls.Time
    NextFeedingCode int  // 0=Late, 1=in time, 2=future, 3=upcoming
}
```

#### `FeedingByZoneMap`
```go
type FeedingByZoneMap map[models.AnimalViewKey]([]AnimalFeeding)
```
- Method: `OrderedKeys() []models.AnimalViewKey` - Returns sorted zone keys

### Constants

```go
const FEEDING_SQL = "..." // SQL to fetch animals needing feeding
const HIGHTIMELIMIT = 2 * time.Hour
const NEARTIMELIMIT = 15 * time.Minute
const feeding_dateFormat = "2006-01-02 15:04"
```

### Functions

#### `calculateFeeding(af AnimalFeeding, now time.Time) AnimalFeeding`
- **Signature**: `func calculateFeeding(af AnimalFeeding, now time.Time) AnimalFeeding`
- **Purpose**: Calculates next feeding time and status code
- **Logic**:
  1. Adjusts feeding start/end to today's date
  2. If last feeding exists, calculates next feeding based on period
  3. Determines status code based on time proximity
- **Returns**: Updated `AnimalFeeding` with `NextFeeding` and `NextFeedingCode`

#### `calculateFeedings(afRaw []AnimalFeeding) (FeedingByZoneMap, error)`
- **Signature**: `func calculateFeedings(afRaw []AnimalFeeding) (FeedingByZoneMap, error)`
- **Purpose**: Calculates feeding schedule for multiple animals
- **Behavior**:
  1. Calculates next feeding for each animal
  2. Filters out animals without valid next feeding
  3. Sorts by next feeding time
  4. Groups by zone

#### `generateAnimalFeedings(c buffalo.Context) (FeedingByZoneMap, error)`
- **Signature**: `func generateAnimalFeedings(c buffalo.Context) (FeedingByZoneMap, error)`
- **Purpose**: Fetches animals from DB and calculates feeding schedule
- **SQL**: Uses `FEEDING_SQL` to find animals with feeding schedules

---

## actions/dashboard.go

**File Path**: `actions/dashboard.go`

Dashboard data aggregation functions.

### SQL Constants

```go
const SQL_CARES_IN_WARNING = "..."  // Cares requiring attention
const SQL_ANIMAL_COUNT_IN_CARE_PER_TYPE = "..."  // Count by type
const SQL_ANIMAL_WITH_TODAY_TREATMENTS = "..."  // Today's treatments
const SQL_ANIMAL_TOBE_FORCEFEED = "..."  // Force feed required
```

### Functions

#### `listOpenCares(c buffalo.Context) ([]models.CareWithAnimalNumber, error)`
- Fetches cares in warning status
- Enriches with animal number info

#### `listAnimalWithTodayTreatments(c buffalo.Context) (*models.Animals, error)`
- Fetches animals with pending treatments for today
- Uses `EnrichAnimalsOptimized`

#### `listAnimalWithForceFeed(c buffalo.Context) (*models.Animals, error)`
- Fetches animals requiring force feeding
- Uses `EnrichAnimalsOptimized`

#### `listAnimalCountPerType(c buffalo.Context) ([]listAnimalCountPerTypeReply, error)`
- Returns count of animals in care grouped by type

#### `listLast24hLogEntries(c buffalo.Context) (models.Logentries, error)`
- Fetches log entries from last 24 hours

#### `DashboardIndex(c buffalo.Context) error`
- Main dashboard handler
- Aggregates all dashboard data and renders view

---

## actions/landing.go

**File Path**: `actions/landing.go`

Landing page functionality.

### Functions

#### `listAnimalWithCleanCage(c buffalo.Context) (map[int]bool, error)`
- **Signature**: `func listAnimalWithCleanCage(c buffalo.Context) (map[int]bool, error)`
- **Purpose**: Finds animals with clean cages in last 24 hours
- **Returns**: Map of animal IDs to boolean (true = clean)

#### `LandingIndex(c buffalo.Context) error`
- Main landing page handler
- Loads all animals in care
- Groups by type and zone
- Checks cage cleanliness

---

## actions/registertable.go

**File Path**: `actions/registertable.go`

Animal register table functionality.

### Types

#### `registerYear`
```go
type registerYear struct {
    Year     string `db:"Year"`
    Selected bool
}
```

### Functions

#### `listRegisterYears(c buffalo.Context) ([]registerYear, error)`
- Fetches distinct years from animals table

#### `RegistertableIndexCSV(c buffalo.Context) error`
- Exports register table as CSV for a specific year

#### `RegistertableIndex(c buffalo.Context) error`
- Displays paginated register table
- Defaults to most recent year

---

## actions/registersnapshot.go

**File Path**: `actions/registersnapshot.go`

Historical register snapshot functionality.

### Constants

```go
const REGISTER_SNAP_SQL = "..."
```
- Finds animals that were in care on a specific date

### Functions

#### `RegistersnapshotIndexCSV(c buffalo.Context) error`
- Exports snapshot as CSV

#### `RegistersnapshotIndex(c buffalo.Context) error`
- Displays snapshot for a specific date
- Defaults to today

---

## actions/reception.go

**File Path**: `actions/reception.go`

Animal reception/intake form setup.

### Functions

#### `ReceptionNew(c buffalo.Context) error`
- Sets up new animal reception form
- Populates selectables for types, ages, zones, entry causes
- Sets defaults:
  - Discovery date: now
  - Country: "Belgique"
  - Intake date: now
  - Default zone
  - Default animal type

---

## actions/maintenance.go

**File Path**: `actions/maintenance.go`

Maintenance utilities (admin only).

### Functions

#### `MaintenanceIndex(c buffalo.Context) error`
- Displays maintenance page
- Requires admin rights

#### `MaintenanceRenumber(c buffalo.Context) error`
- **Signature**: `func MaintenanceRenumber(c buffalo.Context) error`
- **Purpose**: Renumbers all animals by year
- **Behavior**:
  1. Clears year and yearNumber
  2. Sets year from IntakeDate
  3. For each year, assigns sequential yearNumber ordered by intakeDate
- **Requires**: Admin rights
- **Warning**: Heavy operation, modifies all animals

---

## actions/export.go

**File Path**: `actions/export.go`

Export handler wrappers.

### Functions

#### `ExportCsv(c buffalo.Context) error`
- Displays CSV export query list or executes export

#### `ExportExcel(c buffalo.Context) error`
- Displays Excel export query list or executes export

---

## actions/hint.go

**File Path**: `actions/hint.go`

Species hint/lookup functionality.

### Constants

```go
const HINT_SPECIES_DETAILS = "..."
```

### Functions

#### `HintSpeciesDetails(c buffalo.Context) error`
- Returns species details (status, freeable, game, huntable)
- **Query Param**: `q` - species name
- **Returns**: JSON array of species details

---

## actions/switchLanguage.go

**File Path**: `actions/switchLanguage.go`

Language switching functionality.

### Functions

#### `setLang(lang, url string, c buffalo.Context) error`
- **Signature**: `func setLang(lang, url string, c buffalo.Context) error`
- **Purpose**: Sets language cookie and redirects
- **Parameters**:
  - `lang`: Language code (e.g., "en-US", "fr")
  - `url`: Redirect URL (empty = home)
- **Behavior**:
  1. Sets cookie with 265-day expiry
  2. Refreshes translator
  3. Flashes success message
  4. Redirects

#### `SwitchLanguage(c buffalo.Context) error`
- GET handler for language switch
- **Params**: `lang`, `url`

#### `SwitchLanguagePost(c buffalo.Context) error`
- POST handler for language switch
- **Form**: `lang`, `url`

---

## actions/paths.go

**File Path**: `actions/paths.go`

Simple page handler.

### Functions

#### `PathHandler(c buffalo.Context) error`
- Renders paths/index.html

---

## actions/dashboard_weightloss.go

**File Path**: `actions/dashboard_weightloss.go`

Weight loss monitoring functionality.

### Constants

```go
const ANIMALS_WITH_WEIGTHLOSS = "..."
```
- Complex CTE query finding animals with 7%+ weight loss over 10 days

### Types

#### `AnimalWithWeight`
```go
type AnimalWithWeight struct {
    models.Animal
    Weights string `json:"weights" db:"weights"`
}
```

### Functions

#### `listAnimalWithWeightLoss(c buffalo.Context) (*[]AnimalWithWeight, error)`
- **Signature**: `func listAnimalWithWeightLoss(c buffalo.Context) (*[]AnimalWithWeight, error)`
- **Purpose**: Finds animals with significant weight loss
- **Behavior**:
  1. Executes weight loss SQL query
  2. Enriches with animal age information
- **Returns**: Slice of animals with weight history string

---

## actions/app.go

**File Path**: `actions/app.go`

Application setup and routing.

### Global Variables

```go
var ENV = envy.Get("GO_ENV", "development")
var app *buffalo.App
var T *i18n.Translator
var appOnce sync.Once
```

### Functions

#### `App() *buffalo.App`
- **Signature**: `func App() *buffalo.App`
- **Purpose**: Creates and configures Buffalo application
- **Setup**:
  1. Creates app with session name "_creaves_session"
  2. Registers time formats
  3. Adds middleware: paramlogger, CSRF, DB transaction, translations
  4. Sets up routes for all resources
  5. Configures error handlers
- **Returns**: Configured Buffalo app

#### `translations() buffalo.MiddlewareFunc`
- **Signature**: `func translations() buffalo.MiddlewareFunc`
- **Purpose**: Sets up i18n translator
- **Behavior**: Loads locales from embedded filesystem, defaults to "en-US"

#### `forceSSL() buffalo.MiddlewareFunc`
- **Signature**: `func forceSSL() buffalo.MiddlewareFunc`
- **Purpose**: Returns SSL redirect middleware (currently unused)
- **Note**: Commented out in App() - SSL handled by proxy

---

## Key Patterns and Conventions

### Database Access Pattern
All handlers follow this pattern for DB access:
```go
tx, ok := c.Value("tx").(*pop.Connection)
if !ok {
    return fmt.Errorf("no transaction found")
}
```

### Response Pattern
Multi-format responses use responder:
```go
return responder.Wants("html", func(c buffalo.Context) error {
    return c.Render(http.StatusOK, r.HTML("template.html"))
}).Wants("json", func(c buffalo.Context) error {
    return c.Render(200, r.JSON(data))
}).Wants("xml", func(c buffalo.Context) error {
    return c.Render(200, r.XML(data))
}).Respond(c)
```

### Admin Check Pattern
```go
cu := GetCurrentUser(c)
if !cu.Admin {
    return c.Error(http.StatusForbidden, fmt.Errorf("restricted"))
}
```

### Form Selectables Pattern
Reference data is loaded and converted to selectables:
```go
at, err := animalTypes(c)
if err != nil {
    return err
}
c.Set("selectAnimalTypes", animalTypesToSelectables(at))
```

### Cache Invalidation Pattern
Weight loss cache is invalidated when care weights change:
```go
if care.Weight.Valid && len(care.Weight.String) > 0 {
    InvalidateWeightLossCache()
}
```

### Error Handling Pattern
Flash messages for user feedback:
```go
c.Flash().Add("success", T.Translate(c, "key.created.success"))
c.Flash().Add("danger", T.Translate(c, "key.error", data))
```

---

## Performance Notes

1. **EnrichAnimalsOptimized** vs **EnrichAnimals**: The optimized version uses bulk queries (N+1 fix) while the original loads all reference data regardless of need.

2. **Cache Usage**: Weight loss data is cached for 12 hours to reduce expensive SQL queries.

3. **Raw SQL**: Several features use raw SQL for complex queries (weight loss, dashboard, register tables) that are difficult to express in Pop ORM.

4. **Eager Loading**: Individual animal show pages use `tx.Eager()` for deep loading, while list pages use manual enrichment for better control.

---

## Testing

### Feeding Tests
**File**: `actions/feeding_test.go`

Tests `calculateFeeding` function with various scenarios:
- First feeding of the day
- Subsequent feedings
- Time boundary conditions
- Future feeding suppression

Run with: `go test ./actions -run TestCalculateFeeding`
