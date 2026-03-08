# Architecture & Design

## System Architecture

### High-Level Overview

```
┌─────────────────────────────────────────────────────────────┐
│                        Client Browser                        │
│                    (Bootstrap + jQuery)                      │
└─────────────────────┬───────────────────────────────────────┘
                      │ HTTP/HTTPS
                      ▼
┌─────────────────────────────────────────────────────────────┐
│                   Buffalo Web Server                         │
│  ┌──────────────────────────────────────────────────────┐   │
│  │                   Middleware Stack                    │   │
│  │  ┌──────────┐ ┌────────┐ ┌────────┐ ┌────────────┐  │   │
│  │  │ ForceSSL │ │ CSRF   │ │ i18n   │ │ Transaction│  │   │
│  │  └──────────┘ └────────┘ └────────┘ └────────────┘  │   │
│  │  ┌──────────┐ ┌────────┐ ┌────────────────────────┐  │   │
│  │  │ ParamLog │ │ Auth   │ │ Current User           │  │   │
│  │  └──────────┘ └────────┘ └────────────────────────┘  │   │
│  └──────────────────────────────────────────────────────┘   │
│                          │                                   │
│  ┌───────────────────────┴───────────────────────────────┐  │
│  │                   Actions (Controllers)                │  │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌─────────┐  │  │
│  │  │ Animals  │ │  Cares   │ │Treatment │ │Dashboard│  │  │
│  │  └──────────┘ └──────────┘ └──────────┘ └─────────┘  │  │
│  └───────────────────────┬───────────────────────────────┘  │
│                          │                                   │
│  ┌───────────────────────┴───────────────────────────────┐  │
│  │                   Models (Business Logic)              │  │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌─────────┐  │  │
│  │  │ Animal   │ │   Care   │ │Treatment │ │  User   │  │  │
│  │  └──────────┘ └──────────┘ └──────────┘ └─────────┘  │  │
│  └───────────────────────┬───────────────────────────────┘  │
└───────────────────────────┼─────────────────────────────────┘
                            │ Pop ORM
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                      MySQL Database                          │
│  ┌────────┐ ┌────────┐ ┌──────────┐ ┌──────────┐ ┌───────┐ │
│  │animals │ │ cares  │ │treatments│ │discoveries│ │ ...  │ │
│  └────────┘ └────────┘ └──────────┘ └──────────┘ └───────┘ │
└─────────────────────────────────────────────────────────────┘
```

---

## Application Layers

### 1. Presentation Layer

**Location**: `templates/`, `assets/`

**Technologies**:
- Plush templates (`.plush.html`)
- Bootstrap 4.6 (CSS framework)
- jQuery 3.6 (DOM manipulation)
- Select2 (autocomplete)
- Flatpickr (date picker)

**Structure**:
```
templates/
├── application.plush.html    # Main layout
├── _flash.plush.html         # Flash message component
├── animals/
│   ├── index.plush.html      # List view
│   ├── show.plush.html       # Detail view
│   ├── new.plush.html        # Create form
│   ├── edit.plush.html       # Edit form
│   └── _form.plush.html      # Reusable form partial
└── dashboard/
    └── dashboard.plush.html  # Dashboard page
```

**Layout Components**:
- Navigation bar (user menu, language switcher)
- Flash messages (success, danger, warning, info)
- Main content area
- Footer

---

### 2. Controller Layer (Actions)

**Location**: `actions/`

**Responsibilities**:
- HTTP request handling
- Input validation
- Business logic coordination
- Response rendering

**Pattern**: Resource-based controllers

```go
type AnimalsResource struct {
    buffalo.Resource
}

// RESTful actions
func (v AnimalsResource) List(c buffalo.Context) error
func (v AnimalsResource) Show(c buffalo.Context) error
func (v AnimalsResource) New(c buffalo.Context) error
func (v AnimalsResource) Create(c buffalo.Context) error
func (v AnimalsResource) Edit(c buffalo.Context) error
func (v AnimalsResource) Update(c buffalo.Context) error
func (v AnimalsResource) Destroy(c buffalo.Context) error
```

**Response Types**:
- HTML (templates)
- JSON (API)
- XML (API)
- CSV/Excel (exports)
- File downloads

**Key Files**:
| File | Purpose |
|------|---------|
| `app.go` | Application setup, routes, middleware |
| `auth.go` | Authentication handlers |
| `users.go` | User management + auth middleware |
| `animals.go` | Animal CRUD + enrichment logic |
| `cares.go` | Care record management |
| `treatments.go` | Treatment scheduling |
| `dashboard.go` | Dashboard statistics |
| `feeding.go` | Feeding schedule calculations |
| `suggestions.go` | Autocomplete endpoints |
| `render.go` | Template engine configuration |

---

### 3. Business Logic Layer (Models)

**Location**: `models/`

**Responsibilities**:
- Data structure definitions
- Business rules
- Validation logic
- Helper methods

**Pattern**: Active Record (via Pop ORM)

```go
type Animal struct {
    // Fields
    ID         int       `json:"id" db:"id"`
    Species    string    `json:"species" db:"species"`
    
    // Relations
    Animaltype Animaltype `belongs_to:"animaltype"`
    AnimaltypeID uuid.UUID `json:"animaltype_id" db:"animaltype_id"`
    
    // Timestamps
    CreatedAt time.Time `json:"created_at" db:"created_at"`
    UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// Business methods
func (a Animal) YearNumberFormatted() string
func (a Animal) LastWeight() nulls.Int

// Validation
func (a *Animal) Validate(tx *pop.Connection) (*validate.Errors, error)
```

**Key Models**:
| Model | Purpose | Key Methods |
|-------|---------|-------------|
| `Animal` | Animal entity | `YearNumberFormatted()`, `LastWeight()` |
| `Care` | Care record | `DateFormated()` |
| `Treatment` | Treatment schedule | `ScheduleStatus*()`, `IsToday()` |
| `Discovery` | Discovery report | `DateFormated()` |
| `User` | User account | `Create()`, `SetPasswordHash()` |

---

### 4. Data Access Layer (ORM)

**Technology**: Pop v6

**Location**: `models/models.go`, migrations/

**Responsibilities**:
- Database connection management
- Query building
- Result mapping
- Transaction management

**Connection Setup**:
```go
// models/models.go
var DB *pop.Connection

func init() {
    DB, err = pop.Connect(ENV)
    pop.Debug = ENV == "development"
}
```

**Transaction Pattern**:
```go
// actions/app.go
app.Use(popmw.Transaction(models.DB))

// In actions - transaction available in context
tx, ok := c.Value("tx").(*pop.Connection)
```

---

### 5. Middleware Layer

**Location**: `actions/app.go`, `actions/users.go`

**Stack** (in order):
1. **ForceSSL** - HTTPS redirect (production)
2. **ParameterLogger** - Request logging
3. **CSRF** - Token validation
4. **Transaction** - DB transaction per request
5. **i18n** - Language selection
6. **SetCurrentUser** - Load user from session
7. **Authorize** - Require authentication

**Custom Middleware Pattern**:
```go
func SetCurrentUser(next buffalo.Handler) buffalo.Handler {
    return func(c buffalo.Context) error {
        if uid := c.Session().Get("current_user_id"); uid != nil {
            u := &models.User{}
            tx.Find(u, uid)
            c.Set("current_user", u)
        }
        return next(c)
    }
}

func Authorize(next buffalo.Handler) buffalo.Handler {
    return func(c buffalo.Context) error {
        if c.Session().Get("current_user_id") == nil {
            c.Flash().Add("danger", "Unauthorized")
            return c.Redirect(302, "/auth/new")
        }
        return next(c)
    }
}
```

---

## Data Flow

### Typical Request Flow

```
1. Browser sends HTTP request
         ↓
2. Buffalo router matches route
         ↓
3. Middleware stack executes
   - CSRF validation
   - Transaction start
   - Language detection
   - User authentication
         ↓
4. Action handler executes
   - Bind request data
   - Query database via models
   - Apply business logic
         ↓
5. Render response
   - Select template
   - Pass data to template
   - Generate HTML
         ↓
6. Transaction commit/rollback
         ↓
7. Send HTTP response
```

### Example: Create Animal

```
POST /animals
├─ Request Body:
│   species: "Eagle"
│   animaltype_id: "uuid..."
│   animalage_id: "uuid..."
│   ...
│
├─ Middleware:
│   ✓ CSRF token validated
│   ✓ Transaction started
│   ✓ User authenticated
│
├─ AnimalsResource.Create():
│   1. Bind form data to Animal struct
│   2. Set year/yearNumber
│   3. Validate Discovery (nested)
│   4. Validate & Create Animal
│   5. Commit transaction
│
└─ Response:
    302 Redirect to /animals/{id}
    Flash: "Animal created successfully"
```

---

## Key Design Patterns

### 1. Resource Pattern

Buffalo's standard CRUD pattern:

```go
type [Resource]Resource struct {
    buffalo.Resource
}

// Implements: List, Show, New, Create, Edit, Update, Destroy
```

**Benefits**:
- Consistent structure
- Auto-generated by Buffalo
- Easy to understand

### 2. Enrichment Pattern

Load related data efficiently:

```go
// Bad: N+1 queries
for _, animal := range animals {
    tx.Find(&animal.Animaltype, animal.AnimaltypeID)
}

// Good: Bulk load
func EnrichAnimalsOptimized(a *models.Animals, c buffalo.Context) {
    // 1. Collect all IDs
    animaltypeIDs := collectIDs(animals)
    
    // 2. Single query
    animaltypes := models.Animaltypes{}
    tx.Where("id IN (?)", animaltypeIDs).All(&animaltypes)
    
    // 3. Map to animals
    for i := range *a {
        (*a)[i].Animaltype = animaltypeMap[(*a)[i].AnimaltypeID]
    }
}
```

### 3. Bitmap Pattern

Efficient boolean flag storage:

```go
// Treatment schedule
const (
    Treatement_MORNING = 1  // 001
    Treatement_NOON    = 2  // 010
    Treatement_EVENING = 4  // 100
)

// Set bits
timebitmap = Treatement_MORNING | Treatement_EVENING  // 101 = 5

// Check bit
if timebitmap & Treatement_MORNING > 0 {
    // Morning treatment required
}

// Set done
timedonebitmap |= Treatement_MORNING
```

### 4. Response Negotiation Pattern

Support multiple response formats:

```go
return responder.Wants("html", func(c buffalo.Context) error {
    return c.Render(200, r.HTML("/animals/index.plush.html"))
}).Wants("json", func(c buffalo.Context) error {
    return c.Render(200, r.JSON(animals))
}).Wants("xml", func(c buffalo.Context) error {
    return c.Render(200, r.XML(animals))
}).Respond(c)
```

### 5. Flash Message Pattern

User feedback across redirects:

```go
// Set flash
c.Flash().Add("success", T.Translate(c, "animal.created.success"))

// In template
<%= if (flash["success"]) { %>
  <div class="alert alert-success"><%= flash["success"] %></div>
<% } %>
```

---

## Database Design

### Entity Relationship Diagram

```
┌─────────────┐       ┌──────────────┐
│ animaltypes │       │  animalages  │
└──────┬──────┘       └──────┬───────┘
       │                     │
       │ 1                   │ 1
       │                     │
       ▼ *                   ▼ *
┌─────────────────────────────────────┐
│              animals                │
└──┬──────┬───────┬───────┬──────────┘
   │      │       │       │
   │ 1    │ 1     │ 1     │ 1
   │      │       │       │
   ▼ *    ▼ *     ▼ *     ▼ *
┌────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐
│ cares  │ │treatments│ │vet_visits│ │ outtakes │
└────────┘ └──────────┘ └──────────┘ └────┬─────┘
                                          │
                                          │ 1
                                          │
                                          ▼ *
                                     ┌────────────┐
                                     │outtaketypes│
                                     └────────────┘

┌─────────────┐       ┌──────────────┐
│ discoverers │       │ entry_causes │
└──────┬──────┘       └──────┬───────┘
       │                     │
       │ 1                   │ 1
       │                     │
       ▼ *                   ▼ *
┌─────────────────────────────────────┐
│            discoveries              │
└─────────────────┬───────────────────┘
                  │
                  │ 1
                  │
                  ▼ *
             ┌──────────┐
             │ animals  │ (see above)
             └──────────┘
```

### Key Relationships

| Parent | Child | Type | FK Column |
|--------|-------|------|-----------|
| animaltype | animals | 1:* | animaltype_id |
| animalage | animals | 1:* | animalage_id |
| discovery | animals | 1:* | discovery_id |
| intake | animals | 1:* | intake_id |
| animals | cares | 1:* | animal_id |
| animals | treatments | 1:* | animal_id |
| animals | vet_visits | 1:* | animal_id |
| outtake | animals | 1:1 | outtake_id |
| caretype | cares | 1:* | type_id |
| discoverer | discoveries | 1:* | discoverer_id |

### Indexes

**Performance Indexes**:
```sql
-- Animals
CREATE INDEX idx_animals_outtake_id ON animals(outtake_id);
CREATE INDEX idx_animals_zone ON animals(zone);
CREATE INDEX idx_animals_cage ON animals(cage);
CREATE UNIQUE INDEX idx_animals_year_number ON animals(year, yearNumber);

-- Cares
CREATE INDEX idx_cares_date ON cares(date);
CREATE INDEX idx_cares_animal_date ON cares(animal_id, date);
CREATE INDEX idx_cares_type ON cares(type_id);

-- Treatments
CREATE INDEX idx_treatments_date ON treatments(date);
CREATE INDEX idx_treatments_animal_date ON treatments(animal_id, date);

-- Discoveries
CREATE INDEX idx_discoveries_location ON discoveries(location);
```

---

## Security Architecture

### Authentication Flow

```
1. User submits login form
         ↓
2. AuthCreate() handler
   - Find user by login
   - Verify password (bcrypt)
   - Check approved status
         ↓
3. Create session
   c.Session().Set("current_user_id", user.ID)
         ↓
4. Redirect to dashboard
         ↓
5. Subsequent requests
   - SetCurrentUser middleware loads user
   - Authorize middleware checks session
```

### Authorization Model

**User Roles**:
- **Admin**: Full access
- **Standard**: Regular access
- **Shared**: Limited access (flag for future use)

**Protection Patterns**:
```go
// Require authentication (applied globally)
app.Use(Authorize)

// Skip for public routes
auth.Middleware.Skip(Authorize, AuthLanding, AuthNew, AuthCreate)

// Admin-only check (in action)
if !GetCurrentUser(c).Admin {
    return c.Error(http.StatusForbidden, fmt.Errorf("Admin required"))
}
```

### CSRF Protection

```go
// Enabled in app.go
app.Use(csrf.New)

// In templates - automatic token generation
<form>
    <%= csrf() %>
    <!-- form fields -->
</form>
```

### Password Security

```go
// Hashing (models/user.go)
bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)

// Verification (actions/auth.go)
bcrypt.CompareHashAndPassword(hash, password)
```

---

## Internationalization (i18n)

### Architecture

```
┌──────────────────────────────────────┐
│         i18n Middleware              │
│  - Detects language from session     │
│  - Falls back to Accept-Language     │
│  - Default: en-US                    │
└─────────────┬────────────────────────┘
              │
              ▼
┌──────────────────────────────────────┐
│         T Translator                 │
│  - Loaded from locales/*.yaml        │
│  - Key-based lookup                  │
│  - Interpolation support             │
└─────────────┬────────────────────────┘
              │
              ▼
┌──────────────────────────────────────┐
│      Templates / Actions             │
│  T.Translate(c, "key.name", params)  │
└──────────────────────────────────────┘
```

### File Structure

```
locales/
├── all.en-us.yaml          # Common translations
├── all.fr.yaml
├── animals.en-us.yaml      # Resource-specific
├── animals.fr.yaml
├── cares.en-us.yaml
├── cares.fr.yaml
└── ...
```

### Translation Format

```yaml
# all.en-us.yaml
- id: welcome_greeting
  translation: "Welcome to Creaves"

- id: animal.created.success
  translation: "Animal {{.AnimalID}} was successfully created"
```

### Usage

```go
// In actions
T.Translate(c, "animal.created.success", map[string]interface{}{
    "AnimalID": animal.ID,
})

// In templates
<%= T.Translate(c, "welcome_greeting") %>
```

---

## Caching Strategy

### Current Implementation

**Weight Loss Cache** (`actions/cache_utils.go`):

```go
var (
    weightLossCache     *[]AnimalWithWeight
    cacheLastUpdate     time.Time
    cacheUpdateInterval = 12 * time.Hour
)

func GetWeightLossData(c buffalo.Context) (*[]AnimalWithWeight, error) {
    // Check cache
    if time.Since(cacheLastUpdate) < cacheUpdateInterval {
        return weightLossCache, nil
    }
    
    // Refresh cache
    newData, err := listAnimalWithWeightLoss(c)
    weightLossCache = newData
    cacheLastUpdate = time.Now()
    return newData, nil
}
```

### Cache Candidates

| Data | TTL | Invalidation |
|------|-----|--------------|
| Weight Loss | 12h | Manual |
| Animal Count by Type | 1h | On animal create/delete |
| Dashboard Stats | 5min | On care/treatment change |
| Reference Data (types) | 24h | Manual |

### Future Improvements

```go
// Consider Redis for multi-instance deployments
// Add cache invalidation hooks
// Implement cache warming on startup
```

---

## Error Handling

### Error Flow

```
Error occurs in action
         ↓
Return error to Buffalo
         ↓
Buffalo error handler
         ↓
Custom 500 handler (production)
  - Log error
  - Show friendly message
         ↓
Render error page
```

### Custom Error Handler

```go
// actions/app.go
if ENV != "development" {
    app.ErrorHandlers[500] = func(status int, err error, c buffalo.Context) error {
        c.Flash().Add("danger", err.Error())
        return c.Render(status, r.HTML("/oops/oops.plush.html"))
    }
}
```

### Error Types

| Type | HTTP Code | Handling |
|------|-----------|----------|
| Not Found | 404 | `c.Error(http.StatusNotFound, err)` |
| Forbidden | 403 | `c.Error(http.StatusForbidden, err)` |
| Bad Request | 400 | `c.Error(http.StatusBadRequest, err)` |
| Server Error | 500 | Return error or panic |

---

## Testing Architecture

### Test Structure

```
actions/
├── animals_test.go
├── auth_test.go
└── ...

models/
├── animal_test.go
└── ...
```

### Test Suite Setup

```go
package actions

import (
    "testing"
    "github.com/gobuffalo/suite/v4"
)

type ActionSuite struct {
    suite.App
}

func Test_ActionSuite(t *testing.T) {
    suite.Run(t, &ActionSuite{
        App: newApp(),
    })
}
```

### Test Types

1. **Unit Tests**: Model validation, business logic
2. **Integration Tests**: Full request/response cycle
3. **API Tests**: JSON/XML endpoints

---

## Deployment Architecture

### Docker Deployment

```dockerfile
# Multi-stage build
FROM golang AS builder
# Build binary with all assets

FROM alpine
# Copy binary
# Run with minimal footprint
```

### Production Stack

```
                    ┌─────────────┐
                    │   Nginx     │
                    │  (SSL/TLS)  │
                    └──────┬──────┘
                           │
                    ┌──────▼──────┐
                    │   Creaves   │
                    │   Buffalo   │
                    │   Server    │
                    └──────┬──────┘
                           │
                    ┌──────▼──────┐
                    │    MySQL    │
                    │  Database   │
                    └─────────────┘
```

### Environment Configuration

```bash
# Production environment
GO_ENV=production
DATABASE_URL=mysql://user:pass@host:3306/creaves
SESSION_SECRET=secure-random-string
PORT=3000
ADDR=0.0.0.0
```

---

## Performance Considerations

### Bottlenecks

1. **N+1 Queries**: Fixed with bulk loading
2. **Complex Dashboard Queries**: Cached
3. **Template Rendering**: Pre-compiled in production
4. **Asset Loading**: Webpack bundling + CDN

### Optimization Strategies

1. **Database**: Indexes, query optimization
2. **Application**: Caching, connection pooling
3. **Frontend**: Minification, lazy loading
4. **Infrastructure**: CDN, load balancing

---

## Monitoring & Observability

### Current State

- Basic request logging
- Error flash messages
- Manual log inspection

### Recommended Additions

1. **Structured Logging**: Logrus with JSON format
2. **Metrics**: Prometheus endpoint
3. **Tracing**: OpenTelemetry
4. **Error Tracking**: Sentry integration
5. **Health Checks**: `/health` endpoint

---

*This document provides architectural overview. For implementation details, see source code and `docs/README.md`.*
