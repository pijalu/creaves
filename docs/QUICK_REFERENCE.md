# Quick Reference Guide for AI Agents

## 🚀 Essential Commands

### Development
```bash
buffalo dev              # Start development server (http://localhost:3000)
buffalo test             # Run tests
buffalo pop migrate      # Run database migrations
buffalo task             # List available CLI tasks
```

### Database
```bash
buffalo pop create       # Create database
buffalo pop drop         # Drop database
buffalo pop migrate      # Run migrations
buffalo pop migrate reset  # Reset all migrations
buffalo pop seed         # Seed database
```

### Build & Deploy
```bash
buffalo build            # Build production binary
buffalo build --webpack  # Build with assets
docker build -t creaves . # Build Docker image
```

---

## 📁 File Locations Cheat Sheet

| Task | File Location |
|------|---------------|
| Add route | `actions/app.go` |
| Add handler | `actions/[resource].go` |
| Add model | `models/[entity].go` |
| Add template | `templates/[resource]/[view].plush.html` |
| Add translation | `locales/[resource].[lang].yaml` |
| Add migration | `migrations/[timestamp]_[name].up.fizz` |
| Add seed data | `grifts/[name].go` |
| Add CSS | `assets/css/` |
| Add JavaScript | `assets/js/` |

---

## 🔑 Key Patterns

### Standard Resource Pattern

Every resource follows this pattern:

```go
// actions/animals.go
type AnimalsResource struct {
    buffalo.Resource
}

func (v AnimalsResource) List(c buffalo.Context) error {
    tx, _ := c.Value("tx").(*pop.Connection)
    animals := &models.Animals{}
    tx.All(animals)
    return c.Render(200, r.HTML("/animals/index.plush.html"))
}

func (v AnimalsResource) Show(c buffalo.Context) error {
    tx, _ := c.Value("tx").(*pop.Connection)
    animal := &models.Animal{}
    tx.Find(animal, c.Param("animal_id"))
    return c.Render(200, r.HTML("/animals/show.plush.html"))
}

func (v AnimalsResource) Create(c buffalo.Context) error {
    animal := &models.Animal{}
    c.Bind(animal)
    tx, _ := c.Value("tx").(*pop.Connection)
    tx.Create(animal)
    return c.Redirect(302, "/animals/%v", animal.ID)
}

func (v AnimalsResource) Update(c buffalo.Context) error {
    animal := &models.Animal{}
    tx, _ := c.Value("tx").(*pop.Connection)
    tx.Find(animal, c.Param("animal_id"))
    c.Bind(animal)
    tx.Update(animal)
    return c.Redirect(302, "/animals/%v", animal.ID)
}

func (v AnimalsResource) Destroy(c buffalo.Context) error {
    tx, _ := c.Value("tx").(*pop.Connection)
    animal := &models.Animal{}
    tx.Find(animal, c.Param("animal_id"))
    tx.Destroy(animal)
    return c.Redirect(302, "/animals")
}
```

### Model Pattern

```go
// models/animal.go
type Animal struct {
    ID         int       `json:"id" db:"id"`
    Name       string    `json:"name" db:"name"`
    CreatedAt  time.Time `json:"created_at" db:"created_at"`
    UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
    
    // Relations
    Category   Category  `belongs_to:"category"`
    CategoryID uuid.UUID `json:"category_id" db:"category_id"`
}

// Validation
func (a *Animal) Validate(tx *pop.Connection) (*validate.Errors, error) {
    return validate.NewErrors(), nil
}
```

### Template Pattern

```plush
<%# templates/animals/index.plush.html %>
<h1><%= T.Translate(c, "animals.index") %></h1>

<%= paginator(pagination) %>

<table>
  <thead>
    <tr>
      <th>ID</th>
      <th>Species</th>
      <th>Actions</th>
    </tr>
  </thead>
  <tbody>
    <%= for (animal) in animals { %>
      <tr>
        <td><%= animal.YearNumberFormatted() %></td>
        <td><%= animal.Species %></td>
        <td>
          <a href="/animals/<%= animal.ID %>">View</a>
          <a href="/animals/<%= animal.ID %>/edit">Edit</a>
        </td>
      </tr>
    <% } %>
  </tbody>
</table>
```

---

## 🗄️ Database Quick Reference

### Core Tables

| Table | Purpose | Key Fields |
|-------|---------|------------|
| `animals` | Animal records | year, yearNumber, species, cage, zone |
| `cares` | Care records | date, animal_id, type_id, weight, note |
| `treatments` | Medical treatments | date, drug, dosage, timebitmap |
| `discoveries` | Discovery reports | location, date, entry_cause_id |
| `intakes` | Intake records | date, remarks |
| `outtakes` | Outtake records | date, type_id, location |

### Common Queries

```go
// Get all animals
animals := &models.Animals{}
tx.All(animals)

// Get animal by ID
animal := &models.Animal{}
tx.Find(animal, id)

// Get animals with pagination
q := tx.PaginateFromParams(c.Params())
q.All(animals)

// Get animals with conditions
tx.Where("cage = ?", "A1").All(animals)

// Get with relations
tx.Eager("Animaltype", "Discovery").Find(animal, id)

// Raw SQL
tx.RawQuery("SELECT * FROM animals WHERE year = ?", 2024).All(animals)
```

---

## 🎯 Common Tasks

### Create New Animal

```go
animal := &models.Animal{
    Year:       time.Now().Year(),
    YearNumber: getNextYearNumber(tx),
    Species:    "Eagle",
    AnimaltypeID: animaltype.ID,
    AnimalageID:  animalage.ID,
    DiscoveryID:  discovery.ID,
    IntakeID:     intake.ID,
    IntakeDate:   time.Now(),
}

verrs, err := tx.ValidateAndCreate(animal)
```

### Create Care Record

```go
care := &models.Care{
    AnimalID: animal.ID,
    TypeID:   caretype.ID,
    Date:     models.NowOffset(),
    Weight:   nulls.NewString("1500"),
    Note:     nulls.NewString("Normal feeding"),
}

tx.Create(care)
```

### Create Treatment Schedule

```go
// Create treatment for each day in range
for date := startDate; date.Before(endDate); date = date.AddDate(0, 0, 1) {
    treatment := &models.Treatment{
        AnimalID:       animal.ID,
        Date:           date,
        Drug:           "Antibiotic",
        Dosage:         "0.5ml",
        Timebitmap:     models.TreatmentBoolToBitmap(true, false, true),
        Timedonebitmap: 0,
    }
    tx.Create(treatment)
}
```

### Mark Treatment as Done

```go
treatment.Timedonebitmap |= models.Treatement_MORNING  // Set morning bit
tx.Update(treatment)
```

---

## 🔍 Debugging Tips

### Enable Debug Logging

```go
// In any action
c.Logger().Debugf("Variable value: %v", myVar)
```

### Check Database Queries

```go
// Enable Pop debug in models/models.go
pop.Debug = true  // Shows all SQL queries
```

### Inspect Context

```go
// Check what's in the context
tx := c.Value("tx").(*pop.Connection)
user := c.Value("current_user").(*models.User)
```

### Test API Endpoints

```bash
# Test JSON endpoint
curl http://localhost:3000/animals.json

# Test with auth cookie
curl -b "session_cookie" http://localhost:3000/dashboard

# Test POST
curl -X POST -d "name=value" http://localhost:3000/animals
```

---

## 🚨 Common Errors & Solutions

### "no transaction found"

**Cause**: DB transaction not in context  
**Solution**: Ensure middleware is loaded, check `app.Use(popmw.Transaction(models.DB))`

### "invalid login/password"

**Cause**: User not approved or credentials wrong  
**Solution**: Check `users` table, set `approved=true`

### "CSRF token mismatch"

**Cause**: Missing CSRF token in form  
**Solution**: Add `<%= csrf() %>` to forms

### "template not found"

**Cause**: Template path incorrect  
**Solution**: Check path starts with `/`, e.g., `r.HTML("/animals/show.plush.html")`

### Migration errors

**Cause**: Schema mismatch  
**Solution**: `buffalo pop migrate reset && buffalo pop migrate up`

---

## 📊 Dashboard Data

The dashboard (`/dashboard`) shows:

1. **Animals to Treat** - Animals with pending treatments today
2. **Animals to Force Feed** - Animals requiring force feeding
3. **Open Cares** - Cares in warning state
4. **Weight Loss Alerts** - Animals with >7% weight loss
5. **Animal Count by Type** - Statistics per animal type
6. **Recent Log Entries** - System activity (last 24h)

**Key Function**: `DashboardIndex()` in `actions/dashboard.go`

---

## 🍽️ Feeding System

### Feeding Calculation

```go
// Calculate next feeding time
af := AnimalFeeding{
    FeedingStart:  time.Parse("15:04", "08:00"),
    FeedingEnd:    time.Parse("15:04", "18:00"),
    FeedingPeriod: 120, // minutes
    LastFeeding:   nulls.NewTime(lastFeedingTime),
}

result := calculateFeeding(af, time.Now())
// result.NextFeeding = next scheduled time
// result.NextFeedingCode = 0 (late), 1 (soon), 2 (now), 3 (future)
```

### Close Feeding

```go
// POST /feeding/close/:ID/:time/:note
care := &models.Care{
    AnimalID: animalID,
    Date:     parsedTime,
    TypeID:   feedingCareTypeID,
    Note:     nulls.NewString(note),
}
tx.Create(care)
```

---

## 🌐 Internationalization

### Add New Translation

1. Add to `locales/animals.en-us.yaml`:
```yaml
- id: animal.my_custom_message
  translation: "My custom message"
```

2. Add to `locales/animals.fr.yaml`:
```yaml
- id: animal.my_custom_message
  translation: "Mon message personnalisé"
```

3. Use in template:
```plush
<%= T.Translate(c, "animal.my_custom_message") %>
```

4. Use in action:
```go
c.Flash().Add("success", T.Translate(c, "animal.my_custom_message"))
```

---

## 🔐 Authentication

### Check if User is Logged In

```go
user := actions.GetCurrentUser(c)
if user == nil {
    // Not logged in
}
```

### Check Admin Rights

```go
user := actions.GetCurrentUser(c)
if !user.Admin {
    return c.Error(http.StatusForbidden, fmt.Errorf("Admin required"))
}
```

### Create User

```go
user := &models.User{
    Login:    "newuser",
    Password: "password123",
    Admin:    false,
    Approved: false,  // Must be approved by admin
}
verrs, err := user.Create(tx)
```

---

## 📦 Data Seeding

### Run Seed Tasks

```bash
# All seeds
buffalo task init

# Individual seeds
buffalo task create_admin
buffalo task create_animaltypes
buffalo task create_caretype
buffalo task create_drugs
buffalo task create_species
```

### Add New Seed Data

Create `grifts/create_[entity].go`:

```go
package grifts

import (
    "creaves/models"
    "github.com/gobuffalo/buffalo"
)

func init() {
    buffalo.Grifts(actions.App())
    
    buffalo.Group("task", func() {
        buffalo.Task("create_mydata", func(c *buffalo.Context) error {
            tx, _ := models.DB.Connect()
            
            // Create data
            data := &models.MyModel{Name: "Test"}
            tx.Create(data)
            
            return nil
        })
    })
}
```

---

## 🧪 Testing Strategy

### Test Structure

```go
package actions

func (suite *ActionSuite) Test_Animals_List() {
    // Setup
    animal := &models.Animal{Species: "Eagle"}
    suite.DB.Create(animal)
    
    // Execute
    res := suite.HTML().Get("/animals")
    
    // Assert
    suite.Equal(200, res.Code)
    suite.Contains(res.Body.String(), "Eagle")
}
```

### Run Specific Test

```bash
go test -run Test_Animals_List ./actions
```

---

## 📝 Checklist for New Features

- [ ] Create model in `models/`
- [ ] Create migration with `buffalo pop generate migration`
- [ ] Create action in `actions/`
- [ ] Create templates in `templates/[resource]/`
- [ ] Add translations in `locales/`
- [ ] Add route in `actions/app.go`
- [ ] Add to navigation (if needed)
- [ ] Write tests
- [ ] Update documentation

---

## 🎨 Frontend Components

### Available Libraries

- **Bootstrap 4.6** - UI framework
- **jQuery 3.6** - DOM manipulation
- **Select2** - Enhanced select dropdowns
- **Flatpickr** - Date/time picker
- **Bootstrap Table** - Enhanced tables

### Use Select2

```javascript
$('.select2').select2({
    placeholder: "Select an option",
    allowClear: true
});
```

### Use Flatpickr

```javascript
flatpickr("#dateField", {
    dateFormat: "Y-m-d",
    defaultDate: "today"
});
```

---

## 📈 Performance Tips

### Use Bulk Loading

```go
// ❌ Bad - N+1 queries
for _, animal := range animals {
    tx.Find(&animal.Animaltype, animal.AnimaltypeID)
}

// ✅ Good - Single query
animaltypeIDs := []uuid.UUID{...}
animaltypes := models.Animaltypes{}
tx.Where("id IN (?)", animaltypeIDs).All(&animaltypes)
```

### Add Indexes

```sql
CREATE INDEX idx_animals_outtake ON animals(outtake_id);
CREATE INDEX idx_cares_animal_date ON cares(animal_id, date);
CREATE INDEX idx_treatments_animal_date ON treatments(animal_id, date);
```

### Cache Expensive Queries

```go
// See actions/cache_utils.go for pattern
var cache *[]Result
var cacheLastUpdate time.Time

func getData(c buffalo.Context) (*[]Result, error) {
    if time.Since(cacheLastUpdate) < 12*time.Hour {
        return cache, nil
    }
    // Fetch fresh data
    cache = fetchData()
    cacheLastUpdate = time.Now()
    return cache, nil
}
```

---

*For more details, see `docs/README.md` and `TODO.md`*
