# AI Agent Onboarding Guide

## 🎯 Purpose

This guide helps AI agents quickly understand and work with the Creaves codebase with minimal investigation time.

---

## 📚 Documentation Map

| Document | Purpose | When to Read |
|----------|---------|--------------|
| `docs/README.md` | Project overview, quick start | First time setup |
| `docs/QUICK_REFERENCE.md` | Commands, patterns, cheat sheets | Daily work |
| `docs/ARCHITECTURE.md` | System design, patterns | Understanding structure |
| `docs/BUSINESS_LOGIC.md` | Domain knowledge, workflows | Implementing features |
| `TODO.md` | Improvement backlog | Planning work |
| `docs/AI_AGENT_GUIDE.md` | This document | Working on tasks |

---

## 🚀 Quick Start (5 Minutes)

### 1. Understand the Domain

Creaves is a **wildlife sanctuary management system** for tracking rescued animals:

```
Discovery → Intake → Care/Treatment → Outtake
(Found)   (Admit)  (Rehabilitate)   (Release/Transfer/Death)
```

**Key Concepts**:
- Animals identified by year/number (e.g., "245/24")
- Daily care records with weight tracking
- Medical treatments with bitmap scheduling
- Feeding schedule calculations
- Multi-language (EN/FR)

### 2. Project Structure

```
creaves/
├── actions/           # HTTP handlers (controllers)
├── models/            # Database models + business logic
├── templates/         # HTML templates (Plush)
├── locales/           # Translations (en-US, fr-BE)
├── migrations/        # Database migrations
└── docs/              # This documentation
```

### 3. Running the Application

```bash
# Start development server
buffalo dev

# Access at http://localhost:3000
# Default admin: admin / admin
```

### 4. Common Patterns

**Resource Pattern** (CRUD):
```go
type AnimalsResource struct { buffalo.Resource }

func (v AnimalsResource) List(c buffalo.Context) error
func (v AnimalsResource) Show(c buffalo.Context) error
func (v AnimalsResource) Create(c buffalo.Context) error
func (v AnimalsResource) Update(c buffalo.Context) error
func (v AnimalsResource) Destroy(c buffalo.Context) error
```

**Database Access**:
```go
tx, _ := c.Value("tx").(*pop.Connection)
animals := &models.Animals{}
tx.All(animals)
```

**Rendering**:
```go
return c.Render(200, r.HTML("/animals/index.plush.html"))
```

---

## 🔍 Investigation Workflow

When tasked with a new feature or fix:

### Step 1: Identify the Domain Area

**Question**: What business area does this affect?

| Topic | See Document |
|-------|--------------|
| Animal management | `BUSINESS_LOGIC.md` Section 1-2 |
| Care records | `BUSINESS_LOGIC.md` Section 3 |
| Medical treatments | `BUSINESS_LOGIC.md` Section 4 |
| Feeding schedules | `BUSINESS_LOGIC.md` Section 5 |
| Dashboard/Reports | `BUSINESS_LOGIC.md` Section 8-9 |

### Step 2: Find Related Code

**Search Patterns**:

```bash
# Find action file
grep -r "AnimalsResource" actions/

# Find model
ls models/animal*.go

# Find templates
ls templates/animals/

# Find translations
grep -r "animal\." locales/
```

**Key Files by Feature**:

| Feature | Action File | Model | Templates |
|---------|-------------|-------|-----------|
| Animals | `actions/animals.go` | `models/animal.go` | `templates/animals/` |
| Cares | `actions/cares.go` | `models/care.go` | `templates/cares/` |
| Treatments | `actions/treatments.go` | `models/treatment.go` | `templates/treatments/` |
| Dashboard | `actions/dashboard.go` | - | `templates/dashboard/` |
| Feeding | `actions/feeding.go` | - | `templates/feeding/` |

### Step 3: Understand the Flow

**Typical Request Flow**:
```
Route (app.go) → Middleware → Action → Model → Database
                                           ↓
                                    Template ← Response
```

**Trace Example**: Creating an animal

1. Route: `app.Resource("/animals", AnimalsResource{})`
2. Action: `AnimalsResource.Create()`
3. Model: `tx.ValidateAndCreate(animal)`
4. Template: Redirect to `/animals/{id}`
5. Show: `AnimalsResource.Show()` → `templates/animals/show.plush.html`

### Step 4: Check for Existing Patterns

**Before implementing, check**:
- How do similar resources work?
- Is there a helper function?
- Are there translations needed?
- Is there test coverage?

**Example**: Adding a new CRUD resource
1. Copy structure from existing resource (e.g., `animals.go`)
2. Follow same pattern (List, Show, Create, Update, Destroy)
3. Use same template structure
4. Add translations

---

## 🛠️ Common Tasks

### Task: Add a New Field to Animal

**Steps**:

1. **Database Migration**:
```bash
buffalo pop generate migration add_field_to_animals
```

Edit migration file:
```fizz
// migrations/[timestamp]_add_field_to_animals.up.fizz
add_column("animals", "new_field", "string", {"null": true})
```

2. **Update Model** (`models/animal.go`):
```go
type Animal struct {
    // ... existing fields
    NewField nulls.String `json:"new_field" db:"new_field"`
}
```

3. **Update Form** (`templates/animals/_form.plush.html`):
```plush
<div class="form-group">
    <%= f.Label("new_field") %>
    <%= f.InputField("new_field", {class:"form-control"}) %>
</div>
```

4. **Add Translations** (`locales/animals.en-us.yaml`):
```yaml
- id: animal.new_field
  translation: "New Field"
```

5. **Run Migration**:
```bash
buffalo pop migrate
```

### Task: Add a New Page

**Steps**:

1. **Create Action** (`actions/myfeature.go`):
```go
package actions

func MyFeatureIndex(c buffalo.Context) error {
    return c.Render(200, r.HTML("/myfeature/index.plush.html"))
}
```

2. **Create Template** (`templates/myfeature/index.plush.html`):
```plush
<h1>My Feature</h1>
<!-- content -->
```

3. **Add Route** (`actions/app.go`):
```go
app.GET("/myfeature", MyFeatureIndex)
```

4. **Add to Navigation** (`templates/application.plush.html`):
```plush
<li><a href="/myfeature">My Feature</a></li>
```

### Task: Add Business Logic

**Example**: Add validation

1. **Model Validation** (`models/animal.go`):
```go
func (a *Animal) Validate(tx *pop.Connection) (*validate.Errors, error) {
    return validate.Validate(
        &validators.StringIsPresent{Field: a.Species, Name: "Species"},
        &validators.IntIsPresent{Field: a.AnimaltypeID, Name: "AnimaltypeID"},
    ), nil
}
```

2. **Action Already Handles Validation**:
```go
// animals.go:Create() - already has validation logic
verrs, err := tx.ValidateAndCreate(animal)
if verrs.HasAny() {
    c.Set("errors", verrs)
    return c.Render(422, r.HTML("/animals/new.plush.html"))
}
```

### Task: Fix a Bug

**Workflow**:

1. **Reproduce the Issue**
   - Run application
   - Follow steps to reproduce
   - Check error message

2. **Find the Code**
   - Error message mentions file/line?
   - Search for related function
   - Check recent changes

3. **Understand the Problem**
   - Read the code
   - Check input data
   - Trace execution flow

4. **Implement Fix**
   - Make minimal change
   - Follow existing patterns
   - Add comment if needed

5. **Test**
   - Reproduce original issue (should be fixed)
   - Test related functionality (no regression)
   - Run existing tests

**Example**: Fix "no transaction found" error

```go
// Problem: Missing transaction check
func MyAction(c buffalo.Context) error {
    tx := c.Value("tx")  // ❌ No type assertion, no check
    tx.All(&items)
}

// Fix: Proper transaction handling
func MyAction(c buffalo.Context) error {
    tx, ok := c.Value("tx").(*pop.Connection)  // ✅
    if !ok {
        return fmt.Errorf("no transaction found")
    }
    tx.All(&items)
}
```

---

## 📝 Implementation Checklist

When implementing a feature:

### Analysis
- [ ] Understand business requirement
- [ ] Identify affected components
- [ ] Check existing similar implementations
- [ ] Review TODO.md for related items

### Implementation
- [ ] Update database (migration if needed)
- [ ] Update model (fields, validation, methods)
- [ ] Update action (handlers, business logic)
- [ ] Update templates (forms, views)
- [ ] Add translations (EN + FR)
- [ ] Update navigation (if new page)

### Quality
- [ ] Follow existing code patterns
- [ ] Remove debug logging
- [ ] Handle errors properly
- [ ] Add comments for complex logic
- [ ] Update documentation if needed

### Testing
- [ ] Test happy path
- [ ] Test error cases
- [ ] Test edge cases
- [ ] Run existing tests
- [ ] Test in browser

### Cleanup
- [ ] Remove commented code
- [ ] Fix formatting (`go fmt`)
- [ ] Check for TODOs created
- [ ] Update git status

---

## 🚨 Common Pitfalls

### 1. Missing Transaction

**Problem**: "no transaction found" error

**Solution**:
```go
tx, ok := c.Value("tx").(*pop.Connection)
if !ok {
    return fmt.Errorf("no transaction found")
}
```

### 2. CSRF Token Missing

**Problem**: Form submission fails silently

**Solution**: Add to form template:
```plush
<%= csrf() %>
```

### 3. N+1 Query Problem

**Problem**: Loading related data in loop

**Bad**:
```go
for _, animal := range animals {
    tx.Find(&animal.Animaltype, animal.AnimaltypeID)  // ❌ Query per animal
}
```

**Good**:
```go
// Bulk load
types := models.Animaltypes{}
tx.Where("id IN (?)", animalTypeIDs).All(&types)  // ✅ Single query
```

### 4. Hard-coded Strings

**Problem**: Text in templates not translatable

**Bad**:
```plush
<h1>Animals</h1>
```

**Good**:
```plush
<h1><%= T.Translate(c, "animals.index") %></h1>
```

### 5. Direct Database Access

**Problem**: Using `models.DB` directly in actions

**Bad**:
```go
models.DB.All(&animals)  // ❌ Bypasses transaction
```

**Good**:
```go
tx, _ := c.Value("tx").(*pop.Connection)
tx.All(&animals)  // ✅ Uses request transaction
```

---

## 🔧 Debugging Techniques

### 1. Logging

```go
// In any action
c.Logger().Debugf("Variable: %v", myVar)
c.Logger().Debugf("Query result count: %d", len(results))
```

### 2. Database Debug Mode

Enable in `models/models.go`:
```go
pop.Debug = true  // Shows all SQL queries
```

### 3. Template Debugging

```plush
<%# Dump variable in template %>
<pre><%= dbgDump(myVar) %></pre>
```

### 4. Browser DevTools

- Check network tab for failed requests
- Check console for JavaScript errors
- Inspect form submissions

### 5. Test with curl

```bash
# Test API endpoint
curl http://localhost:3000/animals.json

# Test with auth
curl -b "session=xxx" http://localhost:3000/dashboard
```

---

## 📊 Data Reference

### Core Entities

| Entity | Purpose | Key Fields |
|--------|---------|------------|
| `animals` | Animal records | year, yearNumber, species, cage, zone |
| `cares` | Care events | date, animal_id, type_id, weight, note |
| `treatments` | Medical treatments | date, drug, dosage, timebitmap |
| `discoveries` | Discovery reports | location, date, discoverer_id |
| `intakes` | Intake records | date, weight, remarks |
| `outtakes` | Outtake records | date, type_id, location |

### Reference Data

| Table | Examples |
|-------|----------|
| `animaltypes` | Eagle, Owl, Sparrow, Duck |
| `animalages` | Baby, Juvenile, Adult |
| `caretypes` | Feeding, Medication, Cleaning |
| `entry_causes` | Orphaned, Injured, Sick |
| `outtaketypes` | Released, Transferred, Deceased |
| `species` | Accipiter nisus, Buteo buteo |

### Status Codes

**Treatment Bitmap**:
- 1 = Morning
- 2 = Noon
- 4 = Evening
- 3 = Morning + Noon
- 5 = Morning + Evening
- 7 = All three

**Feeding Status**:
- 0 = Late
- 1 = Due soon (<15 min)
- 2 = Due now (<2 hours)
- 3 = Future

---

## 🎯 Quick Lookups

### Route to File Mapping

| Route | Action File | Template |
|-------|-------------|----------|
| `/animals` | `actions/animals.go` | `templates/animals/index.plush.html` |
| `/animals/{id}` | `actions/animals.go:Show` | `templates/animals/show.plush.html` |
| `/cares` | `actions/cares.go` | `templates/cares/index.plush.html` |
| `/treatments` | `actions/treatments.go` | `templates/treatments/index.plush.html` |
| `/dashboard` | `actions/dashboard.go` | `templates/dashboard/dashboard.plush.html` |
| `/feeding` | `actions/feeding.go` | `templates/feeding/index.html` |

### Model Relationships

```
Animal
├─ belongs_to Animaltype
├─ belongs_to Animalage
├─ belongs_to Discovery
├─ belongs_to Intake
├─ belongs_to Outtake (optional)
├─ has_many Cares
├─ has_many Treatments
└─ has_many VetVisits

Discovery
└─ belongs_to Discoverer

Care
├─ belongs_to Animal
└─ belongs_to Caretype

Treatment
└─ belongs_to Animal
```

---

## 🤖 AI-Specific Tips

### 1. Use Search Tools

```bash
# Find all references to a function
grep -r "EnrichAnimals" actions/

# Find template usage
grep -r "animals/show" templates/

# Find translation keys
grep -r "animal.created" locales/
```

### 2. Read Related Code

When modifying a function:
1. Read the function
2. Read functions it calls
3. Read functions that call it
4. Check tests

### 3. Follow Conventions

- Naming: `CamelCase` for types, `lowercase` for functions
- Files: One model/action per file
- Structure: Match existing patterns
- Comments: Explain why, not what

### 4. Test Incrementally

Don't make large changes at once:
1. Make small change
2. Test it works
3. Commit
4. Repeat

### 5. Ask for Clarification

If requirements are unclear:
- What's the business goal?
- What are edge cases?
- What's the priority?

---

## 📚 Learning Resources

### Buffalo Framework

- [Official Docs](https://gobuffalo.io/)
- [Getting Started](https://gobuffalo.io/docs/getting-started)
- [Routing](https://gobuffalo.io/docs/routing)
- [Context](https://gobuffalo.io/docs/context)

### Pop ORM

- [Pop Documentation](https://gobuffalo.io/docs/database/intro)
- [Queries](https://gobuffalo.io/docs/database/queries)
- [Migrations](https://gobuffalo.io/docs/database/migrations)

### Plush Templates

- [Plush Documentation](https://gobuffalo.io/docs/templates)
- [Helpers](https://gobuffalo.io/docs/helpers)

### Project-Specific

- `docs/README.md` - Project overview
- `docs/BUSINESS_LOGIC.md` - Domain knowledge
- `docs/ARCHITECTURE.md` - Technical design
- `TODO.md` - Improvement backlog

---

## ✅ First Task Suggestions

To get familiar with the codebase, start with:

1. **Add a field to an existing model** (e.g., add `phone` to `Discoverer`)
2. **Fix a small bug** from TODO.md
3. **Add validation** to an existing model
4. **Create a simple new page** (e.g., info page)
5. **Add a translation** for missing strings

These tasks cover:
- Database migrations
- Model updates
- Template changes
- Translations
- Testing

---

## 🆘 Getting Help

If stuck:

1. **Check Documentation**
   - Search this docs folder
   - Check Buffalo docs
   - Review TODO.md

2. **Search Codebase**
   - Find similar implementations
   - Check how others solved it
   - Look for patterns

3. **Debug Systematically**
   - Add logging
   - Check database state
   - Test in isolation

4. **Ask Questions**
   - What's the exact error?
   - What have you tried?
   - What's expected vs actual?

---

*Remember: Small, incremental changes are better than large, risky ones. Test frequently and follow existing patterns.*

**Last Updated**: 2025-02-25
