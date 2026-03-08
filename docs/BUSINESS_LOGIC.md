# Business Logic Guide

## Domain Overview

Creaves manages the lifecycle of rescued wild animals, primarily birds, from discovery through rehabilitation to final disposition (release, transfer, or death).

### Core Business Processes

```
┌─────────────────────────────────────────────────────────────────┐
│                    Animal Lifecycle                              │
│                                                                  │
│  Discovery → Intake → Rehabilitation → Outtake                  │
│     ↓          ↓           ↓            ↓                        │
│  Found in   Admission  Medical Care  Release/                   │
│  the wild   Processing  & Monitoring Transfer/Death              │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

---

## 1. Animal Discovery Process

### Business Context

When a wild animal is found (injured, orphaned, sick), a **Discovery** record is created capturing:
- Where and when the animal was found
- Who found it (Discoverer)
- Why it needed rescue (Entry Cause)
- Initial circumstances

### Data Model

```go
type Discovery struct {
    ID            uuid.UUID
    Location      string       // Specific location description
    PostalCode    string
    City          string
    Date          time.Time    // When found
    EntryCause    EntryCause   // FK: Why rescued
    EntryCauseID  string       // e.g., "1.1" = "Found orphaned"
    Reason        string       // Brief reason
    Note          string       // Additional details
    Discoverer    Discoverer   // FK: Who found it
    DiscovererID  uuid.UUID
    ReturnHabitat bool         // Plan to release?
    InGarden      bool         // Found in garden?
}
```

### Entry Cause Hierarchy

Entry causes use a hierarchical coding system:

```
1. Found
   1.1 - Orphaned (young without parent)
   1.2 - Injured
   1.3 - Sick
   1.4 - Disabled

2. Confiscated
   2.1 - Illegal captivity
   2.2 - Trade seizure

3. Transfer
   3.1 - From other center
   3.2 - From individual
```

### Business Rules

1. **Discoverer Information**:
   - Must be recorded for legal compliance
   - Contact info required for potential return
   - Some discoverers request animal return

2. **Location Data**:
   - Precise location important for release planning
   - Postal code used for geographic analysis
   - "In garden" flag indicates human proximity

3. **Entry Cause**:
   - Determines care approach
   - Required for statistics reporting
   - Used for subsidy calculations

### Workflow

```
1. Call received about found animal
         ↓
2. Check if discoverer exists (autocomplete)
   - If yes: select existing
   - If no: create new discoverer
         ↓
3. Record discovery details
   - Location, date, circumstances
   - Entry cause classification
         ↓
4. Animal brought to sanctuary
         ↓
5. Create Intake record
         ↓
6. Create Animal record (links to Discovery)
```

---

## 2. Animal Intake Process

### Business Context

When an animal arrives at the sanctuary, it undergoes intake processing:
- Physical examination
- Initial assessment
- Assignment of identification
- Placement in cage/zone

### Animal Identification System

Animals are identified by a **year-based numbering system**:

```
Format: YEARNUMBER/YEAR
Example: 245/24 = 245th animal intake of 2024

Stored as:
- Year: 2024
- YearNumber: 245
```

**Business Benefits**:
- Sequential tracking within year
- Easy verbal communication ("two-four-five of twenty-four")
- Statistical analysis by year
- Unique identification (database constraint)

### Ring/Band System

Some species receive identification rings:

```go
type Animaltype struct {
    ID           uuid.UUID
    Name         string  // "Eagle", "Owl", etc.
    HasRing      bool    // Does this type use rings?
    DefaultSpecies string // Default species for this type
}
```

**Ring Assignment Rules**:
- Birds of prey: Always ringed
- Songbirds: Usually not ringed
- Ring number recorded in `animal.Ring` field

### Intake Record

```go
type Intake struct {
    ID        uuid.UUID
    Date      time.Time   // Intake date
    Weight    string      // Initial weight (grams)
    Remarks   string      // Initial observations
    Cause     string      // Intake reason
}
```

### Initial Assessment

**Physical Exam**:
- Weight measurement
- Injury assessment
- Hydration status
- Body condition score

**Classification**:
- Animal Type (species category)
- Animal Age (baby, juvenile, adult)
- Gender (M/F/Unknown)

### Cage Assignment

Animals are assigned to cages within zones:

```go
type Animal struct {
    Cage  string  // Specific cage identifier
    Zone  string  // Sanctuary zone/area
}
```

**Zone Logic**:
- Zones group related cages (e.g., "Raptor Zone")
- Facilitates feeding rounds
- Enables area-specific management

### Business Rules

1. **Unique Identification**:
   - Year+YearNumber must be unique
   - Auto-incremented on creation
   - Never reused

2. **Species Classification**:
   - Must match known species in database
   - Autocomplete suggests valid options
   - Important for treatment dosing

3. **Default Values**:
   - Default zone assigned automatically
   - Default animal type if unspecified
   - Default country: "Belgique"

---

## 3. Care Management

### Business Context

Daily care is the core operational activity. Each care event is recorded to:
- Track animal progress
- Ensure consistent care
- Meet regulatory requirements
- Support subsidy claims

### Care Record Structure

```go
type Care struct {
    ID        uuid.UUID
    Date      time.Time   // When care provided
    AnimalID  int         // Which animal
    TypeID    uuid.UUID   // Type of care
    Weight    string      // Weight if measured
    Note      string      // Care details
    Clean     bool        // Cage cleaned?
    InWarning bool        // Needs attention?
    LinkToID  uuid.UUID   // Related care (parent)
}
```

### Care Types

Care types define categories of care:

```
Medical Care:
- Medication administration
- Wound treatment
- Injection

Daily Care:
- Feeding
- Water change
- Cage cleaning

Monitoring:
- Weight check
- Behavior observation
- Recovery assessment

Special:
- Cage change (auto-created)
- Feeding schedule update (auto-created)
```

### Warning System

Some care types trigger **warnings** requiring follow-up:

```go
type Caretype struct {
    ID           uuid.UUID
    Name         string
    Warning      bool    // Creates warning state?
    ResetWarning bool    // Clears warning state?
    Type         int     // Category (feeding=1, etc.)
}
```

**Warning Flow**:
```
Care with warning=true created
         ↓
Animal flagged as "needs attention"
         ↓
Shown on dashboard "Open Cares"
         ↓
Follow-up care with reset_warning=true
         ↓
Warning cleared
```

**Example**:
- "Wound treatment" → Warning (needs monitoring)
- "Wound check - healed" → Reset warning

### Weight Monitoring

Weight is critical health indicator:

**Recording**:
- Weight in grams (string to allow "unknown")
- Recorded during care events
- Tracked over time

**Business Rules**:
- Daily weights for critical animals
- Weekly for stable animals
- Alert if >7% loss in 10 days (see Dashboard)

### Cage Change Detection

**Automatic Care Creation**:

When an animal's cage changes:

```go
// animals.go:Update()
if originalCage != animal.Cage {
    care := &models.Care{
        AnimalID: animal.ID,
        Date:     models.NowOffset(),
        Note:     fmt.Sprintf("Cage %s => %s", originalCage, animal.Cage),
        TypeID:   moveCareTypeID,  // "Move" care type
    }
    tx.Create(care)
}
```

**Purpose**:
- Automatic documentation
- Track animal movements
- Audit trail

### Feeding Schedule Updates

Similar to cage changes, feeding schedule changes auto-create care records:

```go
if originalFeeding != animal.Feeding {
    care := &models.Care{
        AnimalID: animal.ID,
        Date:     models.NowOffset(),
        Note:     animal.Feeding,
        TypeID:   feedingCareTypeID,
    }
    tx.Create(care)
}
```

---

## 4. Treatment Management

### Business Context

Medical treatments require precise scheduling and tracking. The system uses a **bitmap-based scheduling system** for efficiency.

### Treatment Model

```go
type Treatment struct {
    ID             uuid.UUID
    Date           time.Time   // Treatment date
    AnimalID       int         // Which animal
    Drug           string      // Medication name
    Dosage         string      // Amount per dose
    Remarks        string      // Special instructions
    Timebitmap     int         // Required doses (bitmask)
    Timedonebitmap int         // Completed doses (bitmask)
}
```

### Bitmap Scheduling System

**Bit Encoding**:
```
Bit 0 (value 1): Morning dose
Bit 1 (value 2): Noon dose
Bit 2 (value 4): Evening dose

Examples:
- 1 (001) = Morning only
- 3 (011) = Morning + Noon
- 5 (101) = Morning + Evening
- 7 (111) = All three doses
```

**Implementation**:
```go
const (
    Treatement_MORNING = 1  // 001
    Treatement_NOON    = 2  // 010
    Treatement_EVENING = 4  // 100
)

// Create bitmap
bitmap := TreatmentBoolToBitmap(true, false, true)  // 101 = 5

// Check if dose required
if treatment.Timebitmap & Treatement_MORNING > 0 {
    // Morning dose required
}

// Mark dose as done
treatment.Timedonebitmap |= Treatement_MORNING
```

### Status Calculation

**Per-Dose Status**:
```go
func (t *Treatment) ScheduleStatusMorning() nulls.Bool {
    if !t.ScheduleRequiredMorning() {
        return nulls.Bool{Valid: false}  // Not required
    }
    done := (t.Timedonebitmap & Treatement_MORNING) > 0
    return nulls.NewBool(done)  // true=done, false=not done
}
```

**Return Values**:
- `nulls.Bool{Valid: false}`: Not required
- `nulls.Bool{Bool: true}`: Required and done
- `nulls.Bool{Bool: false}`: Required but not done

### Treatment Templates

Create multiple treatments from a template:

```go
type TreatmentTemplate struct {
    Dates    string  // Date range
    AnimalID int
    Drug     string
    Dosage   string
    Morning  bool
    Noon     bool
    Evening  bool
}
```

**Generation Logic**:
```go
// treatments.go:Create()
for date := startDate; date.Before(endDate); date = date.AddDate(0, 0, 1) {
    treatment := &models.Treatment{
        Date:       date,
        AnimalID:   template.AnimalID,
        Drug:       template.Drug,
        Dosage:      template.Dosage,
        Timebitmap: TreatmentBoolToBitmap(
            template.Morning,
            template.Noon,
            template.Evening,
        ),
    }
    tx.Create(treatment)
}
```

### Dosage Calculation

**Weight-Based Dosing**:

```go
// suggestions.go:SuggestionsTreatmentDrugDosage()
// Query: SELECT dosage_per_gram FROM drugs WHERE name = ?
dosage := drug.DosagePerGrams * animalWeight

// Example:
// Drug: 0.01 ml/g
// Animal: 500g
// Dose: 5ml
```

### Today's Treatments Dashboard

**Query Logic**:
```sql
SELECT animals.*
FROM animals
WHERE EXISTS(
    SELECT * FROM treatments
    WHERE treatments.animal_id = animals.id
      AND treatments.date >= TODAY
      AND treatments.date < TOMORROW
      AND treatments.timebitmap <> treatments.timedonebitmap
)
```

**Business Purpose**:
- Shows animals needing treatment today
- Highlights incomplete treatments
- Prioritizes daily work

---

## 5. Feeding Schedule System

### Business Context

Different species require different feeding schedules. The system calculates **next feeding times** automatically to ensure animals are fed on time.

### Feeding Parameters

```go
type Animal struct {
    Feeding       string     // Special instructions
    ForceFeed     bool       // Requires force feeding?
    FeedingStart  time.Time  // Window start (e.g., 08:00)
    FeedingEnd    time.Time  // Window end (e.g., 18:00)
    FeedingPeriod int        // Minutes between feedings
}
```

### Feeding Calculation Algorithm

**Location**: `actions/feeding.go:calculateFeeding()`

**Inputs**:
- Feeding window (start/end times)
- Feeding period (interval in minutes)
- Last feeding timestamp
- Current time

**Algorithm**:

```go
func calculateFeeding(af AnimalFeeding, now time.Time) AnimalFeeding {
    // 1. Adjust feeding times to today
    startTime := time.Date(
        now.Year(), now.Month(), now.Day(),
        af.FeedingStart.Hour(), af.FeedingStart.Minute(),
        0, 0, time.Local,
    )
    
    // 2. If no previous feeding, return start time
    if !af.LastFeeding.Valid {
        af.NextFeeding = startTime
        return af
    }
    
    // 3. Calculate heuristic window
    heuristicEnd := af.FeedingEnd.Add(
        -time.Duration(af.FeedingPeriod/2) * time.Minute,
    )
    
    lastFeeding := af.LastFeeding.Time
    
    // 4. Determine next feeding based on last feeding time
    if lastFeeding.After(heuristicEnd) {
        // Last feeding was late - start fresh today
        startTime = startTime.Add(24 * time.Hour)
    } else if lastFeeding.Before(
        startTime.Add(-time.Duration(af.FeedingPeriod) * time.Minute),
    ) {
        // Last feeding was too early - use today's start
        startTime = startTime
    } else {
        // Normal case - add period to last feeding
        startTime = lastFeeding.Add(
            time.Duration(af.FeedingPeriod) * time.Minute,
        )
    }
    
    // 5. Set next feeding
    af.NextFeeding = startTime
    
    // 6. Calculate status code
    if startTime.Before(now.Add(-time.Duration(af.FeedingPeriod/2) * time.Minute)) {
        af.NextFeedingCode = 0  // Late
    } else if startTime.Before(now.Add(-15 * time.Minute)) {
        af.NextFeedingCode = 1  // Due soon
    } else if startTime.Before(now.Add(2 * time.Hour)) {
        af.NextFeedingCode = 2  // Due now
    } else {
        af.NextFeedingCode = 3  // Future
    }
    
    return af
}
```

### Status Codes

| Code | Meaning | Color | Action |
|------|---------|-------|--------|
| 0 | Late (>period/2 overdue) | Red | Feed immediately |
| 1 | Due soon (<15 min) | Orange | Prepare to feed |
| 2 | Due now (<2 hours) | Green | Feed during round |
| 3 | Future (>2 hours) | Gray | No action needed |

### Feeding Page

**URL**: `/feeding`

**Display**:
- Animals grouped by zone
- Sorted by next feeding time
- Color-coded by status

**Close Feeding**:
```go
// POST /feeding/close/:animalID/:time/:note
care := &models.Care{
    AnimalID: animalID,
    Date:     parsedTime,
    TypeID:   feedingCareTypeID,  // Type = 1 (feeding)
    Note:     note,
}
tx.Create(care)
```

---

## 6. Weight Loss Detection

### Business Context

Rapid weight loss is a critical health indicator. The system automatically detects animals losing >7% body weight within 10 days.

### Detection Algorithm

**Location**: `actions/dashboard_weightloss.go`

**SQL Query**:
```sql
WITH ParsedWeights AS (
    SELECT animal_id, date,
           CAST(NULLIF(weight, '') AS DECIMAL(10,0)) AS weight_grams
    FROM cares
    WHERE date >= DATE_SUB(CURDATE(), INTERVAL 10 DAY)
      AND weight IS NOT NULL AND weight <> ''
),
RankedWeights AS (
    SELECT animal_id, date, weight_grams,
           ROW_NUMBER() OVER (
               PARTITION BY animal_id ORDER BY date DESC
           ) AS recent_rank,
           ROW_NUMBER() OVER (
               PARTITION BY animal_id ORDER BY date ASC
           ) AS oldest_rank
    FROM ParsedWeights
),
AnimalsInNeed AS (
    SELECT a.*
    FROM RankedWeights t1
    JOIN RankedWeights t2 ON t1.animal_id = t2.animal_id
    JOIN animals a ON t1.animal_id = a.ID
    WHERE t1.recent_rank = 1      -- Most recent weight
      AND t2.oldest_rank = 1      -- Oldest weight in window
      AND t1.weight_grams <= t2.weight_grams * 0.93  -- 7% loss
      AND a.outtake_id IS NULL    -- Still in care
)
SELECT a.*, GROUP_CONCAT(rw.weight_grams ORDER BY rw.oldest_rank) AS weights
FROM AnimalsInNeed a
JOIN RankedWeights rw ON a.ID = rw.animal_id
GROUP BY a.ID
```

### Business Logic

**Threshold**: 7% weight loss in 10 days

**Rationale**:
- Normal daily fluctuation: 1-3%
- Concerning loss: 5%+
- Critical loss: 7%+ (triggers alert)

**Display**:
- Dashboard section: "Animals with Weight Loss"
- Shows weight trend (e.g., "1500⇨1450⇨1400⇨1350")
- Animal age category shown

**Caching**:
- Results cached for 12 hours
- Manual invalidation on care updates

---

## 7. Outtake Process

### Business Context

Animals leave the sanctuary through various **outtake types**:

```
Release:
- Released to wild (recovered)
- Released to habitat (translocation)

Transfer:
- To another rehabilitation center
- To zoo/sanctuary (permanent care)
- To private keeper (licensed)

Death:
- Euthanasia (humane)
- Found dead
- Died despite treatment

Error:
- Administrative correction
```

### Outtake Record

```go
type Outtake struct {
    ID       uuid.UUID
    Date     time.Time   // Outtake date
    TypeID   uuid.UUID   // Outtake type
    Type     Outtaketype // FK
    Location string      // Release location
    Note     string      // Details
}
```

### Outtake Types

```go
type Outtaketype struct {
    ID       uuid.UUID
    Name     string  // "Released", "Deceased", etc.
    Required bool    // Must provide details?
    Error    bool    // Administrative correction?
}
```

### Business Rules

1. **Release Requirements**:
   - Must be healthy (vet approval)
   - Release location recorded
   - Preferably near discovery location

2. **Transfer Requirements**:
   - Receiving facility details
   - Transport arrangements
   - Legal documentation

3. **Death Documentation**:
   - Cause of death
   - Necropsy if applicable
   - Disposal method

### Administrative Correction

**"Error" Outtake Type**:

Used when animal was admitted in error:
- Duplicate record
- Wrong species identification
- Should not have been admitted

**Process**:
```go
// animals.go:Destroy()
// Instead of deleting, create "error" outtake
ot := &models.Outtaketype{}
tx.Where("error = ?", true).First(ot)

animal.Outtake = &models.Outtake{
    Animal: *animal,
    Date:   models.NowOffset(),
    Type:   *ot,
    TypeID: ot.ID,
}
tx.Create(animal.Outtake)
```

**Rationale**:
- Maintains audit trail
- No hard deletes
- Statistics accuracy

---

## 8. Dashboard & Statistics

### Dashboard Overview

**URL**: `/dashboard`

**Purpose**: Operational overview for daily work

### Dashboard Sections

#### 1. Animals to Treat

**Query**: Animals with pending treatments today

```sql
SELECT animals.*
FROM animals
WHERE EXISTS(
    SELECT * FROM treatments
    WHERE treatments.animal_id = animals.id
      AND treatments.date >= TODAY
      AND treatments.date < TOMORROW
      AND treatments.timebitmap <> treatments.timedonebitmap
)
```

**Display**:
- Animal ID and species
- Treatment details
- Completion status (✓/✗ per dose)

#### 2. Animals to Force Feed

**Query**: Animals with `force_feed = true` and no outtake

```sql
SELECT * FROM animals
WHERE force_feed = true
  AND outtake_id IS NULL
```

**Purpose**:
- Special feeding requirement
- Critical care indicator

#### 3. Open Cares (Warnings)

**Query**: Most recent care per animal where care type has `warning = true`

```sql
SELECT c.*, a.year, a.yearNumber
FROM cares c
JOIN animals a ON c.animal_id = a.id
JOIN caretypes ct ON c.type_id = ct.id AND ct.warning = true
JOIN (
    SELECT animal_id, MAX(date) AS last_date
    FROM cares
    WHERE type_id IN (
        SELECT id FROM caretypes
        WHERE warning = true OR reset_warning = true
    )
    GROUP BY animal_id
) AS last ON c.animal_id = last.animal_id AND c.date = last.last_date
WHERE a.outtake_id IS NULL
ORDER BY c.date DESC
```

**Purpose**:
- Track animals needing follow-up
- Ensure wound monitoring
- Prevent missed checks

#### 4. Weight Loss Alerts

**Query**: See section 6 above

**Purpose**:
- Early health deterioration detection
- Prompt intervention

#### 5. Animal Count by Type

**Query**:
```sql
SELECT animaltypes.name, COUNT(1) AS count
FROM animaltypes
JOIN animals ON animals.animaltype_id = animaltypes.id
WHERE animals.outtake_id IS NULL
GROUP BY animaltypes.name
ORDER BY animaltypes.name
```

**Purpose**:
- Population overview
- Capacity planning
- Statistical reporting

#### 6. Recent Log Entries

**Query**: System logs from last 24 hours

**Purpose**:
- Audit trail
- Recent activity overview
- Troubleshooting

---

## 9. Reporting & Exports

### CSV Export

**URL**: `/export/csv`

**Purpose**: Data extraction for external analysis

**Format**:
- One table per CSV file
- All columns included
- UTF-8 encoding

### Excel Export

**URL**: `/export/excel`

**Purpose**: Formatted reports for authorities

**Features**:
- Multiple sheets
- Formatted headers
- Linked values
- Statistics summaries

**Templates**:
- Annual report
- Subsidy claims
- Veterinary reports

### Registration Tables

**URL**: `/registertable`

**Purpose**: Official registration records

**Data**:
- All animals for selected year
- Intake and outtake details
- Summary statistics

**Export**: CSV format for authorities

---

## 10. User Management & Permissions

### User Roles

```go
type User struct {
    ID       uuid.UUID
    Login    string
    Admin    bool    // Full access
    Approved bool    // Account activated
    Shared   bool    // Future: shared account flag
}
```

### Permission Matrix

| Action | Anonymous | Standard User | Admin |
|--------|-----------|---------------|-------|
| View animals | ✗ | ✓ | ✓ |
| Create animal | ✗ | ✓ | ✓ |
| Edit animal | ✗ | ✓ | ✓ |
| Delete animal | ✗ | ✗ | ✓ |
| View users | ✗ | ✗ | ✓ |
| Create user | ✗ | ✗ | ✓ |
| Approve user | ✗ | ✗ | ✓ |

### Account Approval Flow

```
1. User registers (self-service)
         ↓
2. Account created with approved=false
         ↓
3. Admin reviews account
         ↓
4. Admin sets approved=true
         ↓
5. User can log in
```

**Business Rationale**:
- Prevent unauthorized access
- Verify user identity
- Control system access

---

## 11. Internationalization

### Supported Languages

- **English (en-US)**: Default
- **French (fr-BE)**: Belgium French

### Translation Strategy

**All user-facing text translated**:
- Navigation
- Form labels
- Error messages
- Flash messages
- Reports

**Code unchanged**:
- Database field names
- Internal identifiers
- API endpoints

### Language Selection

```
1. User selects language
         ↓
2. Stored in session
         ↓
3. Middleware sets T translator
         ↓
4. Templates use T.Translate()
```

---

## 12. Compliance & Reporting

### Regulatory Requirements

**Belgian Wildlife Rehabilitation**:
- Record all intakes
- Track all treatments
- Report releases/deaths
- Maintain discoverer records

### Subsidy Reporting

**Data Required**:
- Animal count by type
- Release success rate
- Operational statistics
- Financial data (future)

**Subside Groups**:
```go
type SubsideGroup struct {
    ID   uuid.UUID
    Name string  // e.g., "Birds of Prey"
}
```

Used to categorize animals for subsidy calculations.

---

## Glossary

| Term | Definition |
|------|------------|
| **Animal** | Individual rescued animal |
| **Animaltype** | Category (eagle, owl, sparrow) |
| **Animalage** | Age category (baby, juvenile, adult) |
| **Discovery** | Report of found animal |
| **Discoverer** | Person who found animal |
| **Entry Cause** | Reason for rescue |
| **Intake** | Admission process/record |
| **Outtake** | Departure process/record |
| **Care** | Daily care event |
| **Caretype** | Category of care |
| **Treatment** | Medical treatment schedule |
| **Ring** | Identification band |
| **Cage** | Physical enclosure |
| **Zone** | Group of cages (area) |
| **Force Feed** | Manual feeding required |
| **Warning** | Care requiring follow-up |
| **Bitmap** | Bitmask for treatment schedule |

---

*This document describes business logic. For technical implementation, see source code.*
