# Creaves - Wildlife Sanctuary Management System

## Project Overview

**Creaves** is a web-based wildlife sanctuary management application built with the [Buffalo Framework](https://gobuffalo.io/) (Go) and Bootstrap. It helps manage day-to-day operations of a bird sanctuary, including animal intake, care tracking, treatments, and releases.

**Domain**: Wildlife/Bird Sanctuary Management  
**Primary Use Case**: Tracking rescued animals from discovery through rehabilitation to release or permanent care

---

## Quick Start for AI Agents

### 1. Project Location
```
/Users/muaddib/dev/creaves/
```

### 2. Running the Application

**Development Mode:**
```bash
# Start Buffalo development server (auto-reload)
buffalo dev

# Application will be available at:
http://localhost:3000
```

**Default Admin Credentials** (created by grifts):
- Login: `admin`
- Password: `admin` (or check `grifts/create_admin.go`)

### 3. Key Directories

```
creaves/
├── actions/           # HTTP handlers/controllers (Buffalo "actions")
├── models/            # Database models and business logic
├── templates/         # HTML templates (Plush templating)
├── locales/           # Internationalization files (en-US, fr-BE)
├── migrations/        # Database migrations (Fizz)
├── grifts/            # CLI tasks/data seeding scripts
├── config/            # Configuration files
├── assets/            # Frontend assets (SCSS, JS)
├── public/            # Static files
├── excel/             # Excel export templates
└── docs/              # This documentation
```

### 4. Database

**Type**: MySQL 8.4  
**Default Connection**: `mysql://creaves:creaves@localhost:3306/creaves`

**Run Migrations:**
```bash
buffalo pop migrate
```

**Seed Initial Data:**
```bash
buffalo task init          # Run all initialization tasks
buffalo task create_admin  # Create admin user
```

### 5. Technology Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go 1.18+ (upgrade to 1.21+ recommended) |
| Framework | Buffalo v0.18.9 |
| ORM | Pop v6 |
| Database | MySQL 8.4 |
| Frontend | Bootstrap 4.6, jQuery, Webpack |
| Templates | Plush (Buffalo's templating) |
| i18n | go-i18n (English/French) |

---

## Core Business Concepts

### Animal Lifecycle

```
Discovery → Intake → Care/Treatment → Outtake
    ↓          ↓         ↓              ↓
  Found    Admission  Rehabilitation  Release/
  Report   Processing  & Monitoring   Transfer/Death
```

### Key Entities

1. **Animal** - Central entity representing a rescued animal
2. **Discovery** - Circumstances and location where animal was found
3. **Intake** - Admission details and initial assessment
4. **Care** - Daily care records (feeding, cleaning, weight monitoring)
5. **Treatment** - Medical treatments with scheduling
6. **VeterinaryVisit** - Vet examinations
7. **Outtake** - Final disposition (release, transfer, death)

### Special Features

- **Feeding Schedule Calculator**: Automatically calculates next feeding times based on species-specific schedules
- **Weight Loss Detection**: Alerts when animals lose >7% body weight in 10 days
- **Treatment Bitmap System**: Efficient scheduling for multi-dose treatments (morning/noon/evening)
- **Cage/Zone Management**: Track animal locations within the sanctuary
- **Multi-language Support**: English and French interfaces

---

## Architecture Overview

### Request Flow

```
HTTP Request → Buffalo Router → Middleware → Action → Model → Database
                     ↓                              ↓
              (Auth, CSRF, i18n)              (Pop ORM)
                     ↓                              ↓
              Action → Template → HTML Response
```

### Middleware Stack

1. **ForceSSL** - HTTPS redirect (production only)
2. **ParameterLogger** - Request logging
3. **CSRF Protection** - Cross-site request forgery prevention
4. **Transaction** - Database transaction per request
5. **i18n** - Language selection
6. **SetCurrentUser** - Load user from session
7. **Authorize** - Require authentication

### Directory Structure Deep Dive

```
actions/
├── app.go                 # Main application setup, routes, middleware
├── auth.go                # Authentication (login/logout)
├── users.go               # User management + middleware
├── animals.go             # Animal CRUD + enrichment logic
├── cares.go               # Care records management
├── treatments.go          # Treatment scheduling
├── dashboard.go           # Dashboard with statistics
├── feeding.go             # Feeding schedule calculations
├── suggestions.go         # Autocomplete endpoints
├── export.go              # CSV/Excel exports
└── [resource].go          # One file per resource (28 total)

models/
├── models.go              # DB connection initialization
├── animal.go              # Animal model with business logic
├── care.go                # Care record model
├── treatment.go           # Treatment model + bitmap logic
├── discovery.go           # Discovery report model
├── [entity].go            # One file per entity (24 total)
└── constants.go           # Date formats, utilities

templates/
├── application.plush.html # Main layout
├── [resource]/            # One folder per resource
│   ├── index.plush.html   # List view
│   ├── show.plush.html    # Detail view
│   ├── new.plush.html     # Create form
│   └── edit.plush.html    # Edit form
└── dashboard/             # Special pages
```

---

## Development Guide

### Adding a New Resource

Buffalo uses a resource generator pattern. Most resources follow this structure:

```bash
# Generate a new resource (example)
buffalo generate resource birds name:string species:string
```

**Manual Implementation Pattern:**

1. **Create Model** (`models/bird.go`):
```go
type Bird struct {
    ID        uuid.UUID `json:"id" db:"id"`
    Name      string    `json:"name" db:"name"`
    CreatedAt time.Time `json:"created_at" db:"created_at"`
    UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}
```

2. **Create Migration**:
```bash
buffalo pop generate migration create_birds
```

3. **Create Action** (`actions/birds.go`):
```go
type BirdsResource struct {
    buffalo.Resource
}

func (v BirdsResource) List(c buffalo.Context) error {
    // Implementation
}
```

4. **Create Templates** (`templates/birds/*.plush.html`)

5. **Register Route** (`actions/app.go`):
```go
app.Resource("/birds", BirdsResource{})
```

### Common Patterns

#### Database Queries
```go
// Get DB connection
tx, ok := c.Value("tx").(*pop.Connection)
if !ok {
    return fmt.Errorf("no transaction found")
}

// Simple query
birds := &models.Birds{}
if err := tx.All(birds); err != nil {
    return err
}

// Query with conditions
if err := tx.Where("species = ?", "Eagle").All(birds); err != nil {
    return err
}

// Paginated query
q := tx.PaginateFromParams(c.Params())
if err := q.All(birds); err != nil {
    return err
}
```

#### Rendering Responses
```go
// HTML response
return c.Render(http.StatusOK, r.HTML("/birds/index.plush.html"))

// JSON response
return c.Render(200, r.JSON(birds))

// Redirect with flash
c.Flash().Add("success", "Bird created successfully")
return c.Redirect(http.StatusSeeOther, "/birds/%v", bird.ID)
```

#### Form Binding
```go
bird := &models.Bird{}
if err := c.Bind(bird); err != nil {
    return err
}
```

### Testing

**Run Tests:**
```bash
go test ./...
```

**Current Test Coverage**: Minimal (only feeding calculations)

**Adding Tests:**
```go
// actions/birds_test.go
package actions

func (suite *ActionSuite) Test_Birds_List() {
    res := suite.HTML().Get("/birds")
    suite.Equal(200, res.Code)
}
```

---

## API Reference

### Authentication

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/auth/new` | Login page |
| POST | `/auth/` | Login submission |
| DELETE | `/auth/` | Logout |

### Resources

All resources follow RESTful conventions:

| Method | Endpoint | Action |
|--------|----------|--------|
| GET | `/{resource}` | List all |
| GET | `/{resource}/new` | New form |
| POST | `/{resource}` | Create |
| GET | `/{resource}/{id}` | Show details |
| GET | `/{resource}/{id}/edit` | Edit form |
| PUT | `/{resource}/{id}` | Update |
| DELETE | `/{resource}/{id}` | Delete |

**Available Resources:**
- `/users` - User management
- `/animals` - Animal records
- `/cares` - Care records
- `/treatments` - Medical treatments
- `/veterinaryvisits` - Vet visits
- `/discoveries` - Discovery reports
- `/discoverers` - Discoverer contacts
- `/intakes` - Intake records
- `/outtakes` - Outtake records
- `/animaltypes` - Animal type definitions
- `/animalages` - Age category definitions
- `/caretypes` - Care type definitions
- `/outtaketypes` - Outtake type definitions
- `/traveltypes` - Travel types
- `/travels` - Travel records
- `/drugs` - Medication database
- `/species` - Species database
- `/localities` - Location database
- `/zones` - Sanctuary zones
- `/native_statuses` - Native status definitions
- `/subside_groups` - Subsidy groups
- `/entry_causes` - Entry cause definitions
- `/logentries` - System logs

### Special Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/dashboard` | Dashboard with statistics |
| GET | `/feeding` | Feeding schedule view |
| GET | `/feeding/close` | Mark feeding as done |
| GET | `/reception/new` | Quick animal intake form |
| GET | `/suggestions/*` | Autocomplete endpoints |
| GET | `/export/csv` | Export data as CSV |
| GET | `/export/excel` | Export data as Excel |
| GET | `/registertable` | Registration table report |
| GET | `/registersnapshot` | Registration snapshot |
| GET | `/maintenance/renumber` | Maintenance tasks |

### Autocomplete Endpoints

All return JSON arrays:

```
GET /suggestions/animal_species?q=eagle
GET /suggestions/discovery_location?q=forest
GET /suggestions/outtake_location?q=7000_
GET /suggestions/discoverer_city?q=Mons
GET /suggestions/treatment_drug?q=antibiotic
GET /suggestions/CageWithAnimalInCare?q=A
```

---

## Database Schema

### Core Tables

#### `animals`
Main animal records table.

| Column | Type | Description |
|--------|------|-------------|
| id | INT | Auto-increment primary key |
| year | INT | Year of intake |
| yearNumber | INT | Sequential number within year |
| ring | VARCHAR | Ring/band number |
| species | VARCHAR | Species name |
| gender | VARCHAR | M/F/U |
| cage | VARCHAR | Current cage location |
| zone | VARCHAR | Sanctuary zone |
| feeding | TEXT | Feeding instructions |
| force_feed | BOOLEAN | Requires force feeding |
| feeding_start | TIME | Feeding window start |
| feeding_end | TIME | Feeding window end |
| feeding_period | INT | Minutes between feedings |
| animalage_id | UUID | FK → animalages |
| animaltype_id | UUID | FK → animaltypes |
| discovery_id | UUID | FK → discoveries |
| intake_id | UUID | FK → intakes |
| outtake_id | UUID | FK → outtakes (nullable) |

#### `cares`
Daily care records.

| Column | Type | Description |
|--------|------|-------------|
| id | UUID | Primary key |
| date | DATETIME | Care date/time |
| animal_id | INT | FK → animals |
| type_id | UUID | FK → caretypes |
| weight | VARCHAR | Weight in grams |
| note | TEXT | Care notes |
| clean | BOOLEAN | Cage cleaned |
| in_warning | BOOLEAN | Requires attention |
| link_to_id | UUID | FK → cares (parent care) |

#### `treatments`
Medical treatment schedules.

| Column | Type | Description |
|--------|------|-------------|
| id | UUID | Primary key |
| date | DATETIME | Treatment date |
| animal_id | INT | FK → animals |
| drug | VARCHAR | Medication name |
| dosage | VARCHAR | Dosage amount |
| remarks | TEXT | Additional notes |
| timebitmap | INT | Required doses (bitmask) |
| timedonebitmap | INT | Completed doses (bitmask) |

**Bitmap Encoding:**
- Bit 0 (1): Morning dose
- Bit 1 (2): Noon dose
- Bit 2 (4): Evening dose

Example: `timebitmap = 5` means morning + evening required

#### `discoveries`
Animal discovery reports.

| Column | Type | Description |
|--------|------|-------------|
| id | UUID | Primary key |
| location | VARCHAR | Discovery location |
| postal_code | VARCHAR | Postal code |
| city | VARCHAR | City name |
| date | DATETIME | Discovery date |
| entry_cause_id | VARCHAR | FK → entry_causes |
| reason | TEXT | Discovery reason |
| note | TEXT | Additional notes |
| discoverer_id | UUID | FK → discoverers |
| return_habitat | BOOLEAN | Return to wild planned |
| in_garden | BOOLEAN | Found in garden |

### Reference Tables

- `animaltypes` - Bird type categories (eagle, owl, sparrow, etc.)
- `animalages` - Age categories (baby, juvenile, adult)
- `caretypes` - Care categories (feeding, cleaning, medication)
- `outtaketypes` - Exit categories (released, transferred, deceased)
- `entry_causes` - Reasons for intake (injured, orphaned, sick)
- `species` - Species database
- `drugs` - Medication database
- `zones` - Sanctuary zones
- `localities` - Geographic locations

---

## Business Logic

### Animal Identification

Animals are identified by a year-based numbering system:
- Format: `YEAR/YEARNUMBER` (e.g., `245/24` = 245th animal of 2024)
- Stored as separate `year` and `yearNumber` fields
- Unique constraint on combination

### Feeding Schedule Algorithm

Located in `actions/feeding.go`

**Purpose**: Calculate next feeding time based on:
- Feeding window (start/end times)
- Feeding period (interval in minutes)
- Last feeding timestamp

**Logic**:
1. Adjust feeding times to current date
2. If no previous feeding: return today's start time
3. Calculate heuristic window based on period
4. Determine if next feeding is today or tomorrow
5. Return next scheduled time

**Status Codes**:
- 0: Late (missed feeding)
- 1: Due soon (within 15 min)
- 2: Due now (within 2 hours)
- 3: Future (not due yet)

### Weight Loss Detection

Located in `actions/dashboard_weightloss.go`

**SQL Query**: Identifies animals with >7% weight loss in 10 days

```sql
-- Compares most recent weight with oldest weight in 10-day window
-- Flags if: recent <= old * 0.93 (7% loss)
```

**Cache**: Results cached for 12 hours

### Treatment Scheduling

Located in `actions/treatments.go`

**Template System**: Create multiple treatments from a template
- Select date range
- Choose times (morning/noon/evening)
- Generates individual treatment records

**Progress Tracking**: Bitmap-based completion tracking
- `timebitmap`: Required doses
- `timedonebitmap`: Completed doses
- Visual indicators show completion status

### Cage Change Detection

Located in `actions/animals.go:Update()`

Automatically creates a care record when:
- Animal's cage field changes
- Uses "move" care type if available
- Documents cage change in notes

---

## Internationalization (i18n)

### Supported Languages
- English (en-US)
- French (fr-BE)

### File Structure
```
locales/
├── all.en-us.yaml      # Common translations
├── all.fr.yaml
├── animals.en-us.yaml  # Resource-specific
├── animals.fr.yaml
└── ...
```

### Usage in Templates
```plush
<%= T.Translate(c, "animal.created.success") %>
```

### Usage in Actions
```go
c.Flash().Add("success", T.Translate(c, "animal.updated.success"))
```

### Adding New Translations

1. Add to YAML file:
```yaml
- id: bird.created.success
  translation: "Bird was successfully created"
```

2. French translation (`*.fr.yaml`):
```yaml
- id: bird.created.success
  translation: "L'oiseau a été créé avec succès"
```

---

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `GO_ENV` | Environment (development/production) | `development` |
| `DATABASE_URL` | Database connection string | See database.yml |
| `PORT` | Server port | 3000 |
| `SESSION_SECRET` | Session encryption key | Auto-generated |

### Configuration Files

**`config/buffalo-app.toml`**:
```toml
name = "creaves"
bin = "bin/creaves"
with_pop = true
with_webpack = true
with_docker = true
as_web = true
```

**`database.yml`**:
```yaml
development:
  dialect: "mysql"
  url: "mysql://creaves:creaves@localhost:3306/creaves?parseTime=true"

production:
  dialect: "mysql"
  url: "{{envOr 'DATABASE_URL' '...'}}"
```

---

## Deployment

### Docker Deployment

**Build Image:**
```bash
docker build -t creaves .
```

**Run Container:**
```bash
docker run -p 3000:3000 \
  -e DATABASE_URL="mysql://..." \
  -e GO_ENV=production \
  creaves
```

### Production Checklist

- [ ] Set `GO_ENV=production`
- [ ] Configure production database
- [ ] Set up SSL certificates
- [ ] Enable SSL redirect in `actions/app.go`
- [ ] Configure session secret
- [ ] Set up log rotation
- [ ] Configure backup strategy
- [ ] Set up monitoring (health checks)
- [ ] Review security settings

### Health Check Endpoint

Add to `actions/app.go`:
```go
app.GET("/health", func(c buffalo.Context) error {
    return c.Render(200, r.JSON(map[string]string{
        "status": "ok",
    }))
})
```

---

## Troubleshooting

### Common Issues

**Database Connection Errors:**
```bash
# Check MySQL is running
mysql -u creaves -p

# Verify database exists
mysql -e "SHOW DATABASES;" | grep creaves
```

**Migration Issues:**
```bash
# Reset migrations (WARNING: destructive)
buffalo pop migrate reset
buffalo pop migrate up
```

**Asset Compilation:**
```bash
# Rebuild assets
buffalo build --webpack
```

**Port Already in Use:**
```bash
# Change port
PORT=3001 buffalo dev
```

### Debug Mode

Enable verbose logging in development:
```go
// actions/app.go
app.Use(paramlogger.ParameterLogger)
```

View logs:
```bash
# Buffalo logs appear in terminal
# Check browser console for frontend errors
```

---

## Support & Resources

### Documentation Links
- [Buffalo Framework](https://gobuffalo.io/)
- [Pop ORM](https://gobuffalo.io/docs/database/intro)
- [Plush Templates](https://gobuffalo.io/docs/templates)
- [Go i18n](https://github.com/nicksnyder/go-i18n)

### Code Quality
- See `TODO.md` for improvement backlog
- Run `go vet ./...` for static analysis
- Run `golangci-lint run` for linting

### Contact
- Project: Creaves PoC webapp
- Purpose: Bird sanctuary management
- Repository: Local development

---

## Glossary

| Term | Definition |
|------|------------|
| **Animal** | A rescued animal in the sanctuary's care |
| **Discovery** | Report of an animal found in the wild |
| **Intake** | Process of admitting an animal to the sanctuary |
| **Outtake** | Process of an animal leaving the sanctuary |
| **Care** | Daily care record (feeding, cleaning, etc.) |
| **Treatment** | Medical treatment with scheduled doses |
| **Ring** | Identification band placed on birds |
| **Cage** | Physical enclosure where animal is housed |
| **Zone** | Area of the sanctuary (multiple cages per zone) |
| **Force Feed** | Manual feeding required for weak animals |
| **Bitmap** | Bitmask encoding treatment schedule |

---

*Last Updated: 2025-02-25*  
*Version: 1.0.0*
