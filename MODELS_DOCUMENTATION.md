# Creaves Models Documentation

## Table of Contents

- [Core Models](#core-models)
  - [Animal](#animal)
  - [Discovery](#discovery)
  - [Intake](#intake)
  - [Outtake](#outtake)
  - [Care](#care)
  - [Treatment](#treatment)
  - [VeterinaryVisit](#veterinaryvisit)
  - [Travel](#travel)
- [Reference Models](#reference-models)
  - [AnimalType](#animaltype)
  - [AnimalAge](#animalage)
  - [CareType](#caretype)
  - [OuttakeType](#outtaketype)
  - [TravelType](#traveltype)
  - [Drug](#drug)
  - [Dosage](#dosage)
  - [Species](#species)
  - [Zone](#zone)
  - [Locality](#locality)
  - [EntryCause](#entrycause)
  - [NativeStatus](#nativestatus)
  - [SubsideGroup](#subsidegroup)
  - [Discoverer](#discoverer)
- [User & Auth Models](#user--auth-models)
  - [User](#user)
  - [LogEntry](#logentry)
- [Utility Files](#utility-files)
  - [models.go](#modelsgo)
  - [constants.go](#constantsgo)

---

## Core Models

### Animal

**File:** `models/animal.go`

Central entity representing an animal in the care system.

#### Struct Fields

| Field | Type | DB Column | Tags |
|-------|------|-----------|------|
| ID | `int` | `id` | json:"id" |
| Year | `int` | `year` | json:"Year" |
| YearNumber | `int` | `yearNumber` | json:"YearNumber" |
| Ring | `nulls.String` | `ring` | json:"ring" |
| Species | `string` | `species` | json:"species" |
| Gender | `nulls.String` | `gender` | json:"gender" |
| Cage | `nulls.String` | `cage` | json:"cage" |
| Zone | `nulls.String` | `zone` | json:"zone" |
| Feeding | `nulls.String` | `feeding` | json:"feeding" |
| ForceFeed | `bool` | `force_feed` | json:"forceFeed" |
| FeedingStart | `nulls.Time` | `feeding_start` | json:"feedingStart" |
| FeedingEnd | `nulls.Time` | `feeding_end` | json:"feedingEnd" |
| FeedingPeriod | `int` | `feeding_period` | json:"feedingPeriod" |
| Animalage | `Animalage` | - | belongs_to:"animalage", json:"animalage" |
| AnimalageID | `uuid.UUID` | `animalage_id` | json:"animalage_id" |
| Animaltype | `Animaltype` | - | belongs_to:"animaltype", json:"animaltype" |
| AnimaltypeID | `uuid.UUID` | `animaltype_id` | json:"animaltype_id" |
| Discovery | `Discovery` | - | belongs_to:"discovery", json:"discovery" |
| DiscoveryID | `uuid.UUID` | `discovery_id` | json:"discovery_id" |
| Intake | `Intake` | - | belongs_to:"intake", json:"dssaaintake" |
| IntakeID | `uuid.UUID` | `intake_id` | json:"intake_id" |
| Outtake | `*Outtake` | - | belongs_to:"outtake", json:"outtake,omitempty" |
| OuttakeID | `nulls.UUID` | `outtake_id` | json:"outtake_id" |
| Cares | `[]Care` | - | has_many:"cares", json:"cares,omitempty" |
| Treatments | `Treatments` | - | has_many:"treatments", json:"treatmentes,omitempty" |
| VetVisits | `[]Veterinaryvisit` | - | has_many:"veternaryvisits", json:"veternary_visits,omitempty" |
| IntakeDate | `time.Time` | `IntakeDate` | json:"intakeDate" |
| CreatedAt | `time.Time` | `created_at` | json:"created_at" |
| UpdatedAt | `time.Time` | `updated_at` | json:"updated_at" |

#### Relationships

- **belongs_to**: Animalage, Animaltype, Discovery, Intake, Outtake
- **has_many**: Cares, Treatments, VetVisits

#### Key Methods

- `YearNumberFormatted() string` - Returns formatted year number (e.g., "1/24")
- `ZoneAsString() string` - Returns zone as string
- `FeedingStartFmt() string` - Returns feeding start time formatted (default: "07:00")
- `FeedingEndFmt() string` - Returns feeding end time formatted (default: "22:00")
- `FeedingPeriodHourMinute() string` - Returns feeding period as HH:MM
- `LastWeight() nulls.Int` - Calculates the most recent weight from care records

#### Validation Rules

- `Validate`: Trims string fields (no specific validators)
- `ValidateCreate`: No additional validation
- `ValidateUpdate`: No additional validation

#### Important Business Logic

- Feeding defaults: `DEF_FEEDING_START = "07:00"`, `DEF_FEEDING_END = "22:00"`
- `LastWeight()` iterates through all care records to find the most recent non-empty weight value
- Supports grouping by type (`AnimalsByTypeMap`) and by zone (`AnimalByZoneMap`)

---

### Discovery

**File:** `models/discovery.go`

Records how an animal was discovered/found.

#### Struct Fields

| Field | Type | DB Column | Tags |
|-------|------|-----------|------|
| ID | `uuid.UUID` | `id` | json:"id" |
| Location | `nulls.String` | `location` | json:"location" |
| PostalCode | `nulls.String` | `postal_code` | json:"postal_code" |
| City | `nulls.String` | `city` | json:"city" |
| Date | `time.Time` | `date` | json:"date" |
| EntryCause | `EntryCause` | - | belongs_to:"entry_cause", json:"entry_cause,omitempty" |
| EntryCauseID | `string` | `entry_cause_id` | json:"entry_cause_id" |
| Reason | `nulls.String` | `reason` | json:"reason" |
| Note | `nulls.String` | `note` | json:"note" |
| Discoverer | `Discoverer` | - | belongs_to:"discoverer", json:"discoverer,omitempty" |
| DiscovererID | `uuid.UUID` | `discoverer_id` | json:"discoverer_id" |
| ReturnHabitat | `bool` | `return_habitat` | json:"return_habitat" |
| InGarden | `bool` | `in_garden` | json:"in_garden" |
| CreatedAt | `time.Time` | `created_at"` | json:"created_at" |
| UpdatedAt | `time.Time` | `updated_at"` | json:"updated_at" |

#### Relationships

- **belongs_to**: EntryCause, Discoverer

#### Key Methods

- `DateFormated() string` - Returns formatted date using `DateTimeFormat`

#### Validation Rules

- `Validate`: Date must be present (`TimeIsPresent`)
- `ValidateCreate`: No additional validation
- `ValidateUpdate`: No additional validation

---

### Intake

**File:** `models/intake.go`

Records the initial intake assessment of an animal.

#### Struct Fields

| Field | Type | DB Column | Tags |
|-------|------|-----------|------|
| ID | `uuid.UUID` | `id` | json:"id" |
| Date | `time.Time` | `date` | json:"date" |
| General | `nulls.String` | `general` | json:"general" |
| HasWounds | `bool` | `has_wounds"` | json:"has_wounds" |
| Wounds | `nulls.String` | `wounds"` | json:"wounds" |
| HasParasites | `bool` | `has_parasites"` | json:"has_parasites" |
| Parasites | `nulls.String` | `parasites"` | json:"parasites" |
| Remarks | `nulls.String` | `remarks"` | json:"remarks" |
| CreatedAt | `time.Time` | `created_at"` | json:"created_at" |
| UpdatedAt | `time.Time` | `updated_at"` | json:"updated_at" |

#### Key Methods

- `DateFormated() string` - Returns formatted date

#### Validation Rules

- `Validate`: Trims string fields (no specific validators)
- `ValidateCreate`: No additional validation
- `ValidateUpdate`: No additional validation

---

### Outtake

**File:** `models/outtake.go`

Records when an animal leaves the care center.

#### Struct Fields

| Field | Type | DB Column | Tags |
|-------|------|-----------|------|
| ID | `uuid.UUID` | `id` | json:"id" |
| Animal | `Animal` | - | has_one:"animal", json:"animal,omitempty" |
| Date | `time.Time` | `date` | json:"date" |
| Type | `Outtaketype` | - | belongs_to:"outtaketype", json:"type" |
| TypeID | `uuid.UUID` | `outtaketype_id` | json:"type_id" |
| Location | `nulls.String` | `location` | json:"location" |
| Note | `nulls.String` | `note` | json:"note" |
| CreatedAt | `time.Time` | `created_at"` | json:"created_at" |
| UpdatedAt | `time.Time` | `updated_at"` | json:"updated_at" |

#### Relationships

- **has_one**: Animal
- **belongs_to**: Outtaketype

#### Key Methods

- `IsSelected(value interface{}) bool` - Checks if the given UUID matches TypeID
- `DateFormated() string` - Returns formatted date

#### Validation Rules

- `Validate`: Date must be present (`TimeIsPresent`)
- `ValidateCreate`: No additional validation
- `ValidateUpdate`: No additional validation

---

### Care

**File:** `models/care.go`

Daily care records for animals (feeding, cleaning, weight tracking).

#### Struct Fields

| Field | Type | DB Column | Tags |
|-------|------|-----------|------|
| ID | `uuid.UUID` | `id` | json:"id" |
| Date | `time.Time` | `date"` | json:"date" |
| AnimalID | `int` | `animal_id"` | json:"animal_id" |
| Animal | `Animal` | - | belongs_to:"animal", json:"-" |
| Type | `Caretype` | - | belongs_to:"caretype", json:"type" |
| TypeID | `uuid.UUID` | `type_id"` | json:"type_id" |
| Weight | `nulls.String` | `weight"` | json:"weight" |
| Note | `nulls.String` | `note"` | json:"note" |
| Clean | `nulls.Bool` | `clean"` | json:"clean" |
| InWarning | `nulls.Bool` | `in_warning"` | json:"in_warning" |
| LinkToID | `nulls.UUID` | `link_to_id"` | json:"link_to_id" |
| LinkTo | `*Care` | - | belongs_to:"care", json:"link_to,omitempty" |
| CreatedAt | `time.Time` | `created_at"` | json:"created_at" |
| UpdatedAt | `time.Time` | `updated_at"` | json:"updated_at" |

#### Relationships

- **belongs_to**: Animal, Caretype, Care (self-referential for linked records)

#### Key Methods

- `DateFormated() string` - Returns formatted date

#### Helper Types

- `CareWithAnimalNumber` - Embeds Care with Year and YearNumber fields for reporting

#### Validation Rules

- `Validate`: Date must be present (`TimeIsPresent`)
- `ValidateCreate`: No additional validation
- `ValidateUpdate`: No additional validation

---

### Treatment

**File:** `models/treatment.go`

Medical treatment schedules and records.

#### Struct Fields

| Field | Type | DB Column | Tags |
|-------|------|-----------|------|
| ID | `uuid.UUID` | `id"` | json:"id" |
| Date | `time.Time` | `date"` | json:"date" |
| AnimalID | `int` | `animal_id"` | json:"animal_id" |
| Animal | `*Animal` | - | belongs_to:"animal", json:"-" |
| Drug | `string` | `drug"` | json:"drug" |
| Dosage | `string` | `dosage"` | json:"dosage" |
| Remarks | `nulls.String` | `remarks"` | json:"remarks" |
| Timebitmap | `int` | `timebitmap"` | json:"timebitmap" |
| Timedonebitmap | `int` | `timedonebitmap"` | json:"timedonebitmap" |
| CreatedAt | `time.Time` | `created_at"` | json:"created_at" |
| UpdatedAt | `time.Time` | `updated_at"` | json:"updated_at" |

#### Relationships

- **belongs_to**: Animal

#### Key Methods

- `DateFormated() string` - Returns formatted date using `DateFormat`
- `IsPast() bool` - Check if treatment date is in the past
- `IsToday() bool` - Check if treatment is scheduled for today
- `IsFuture() bool` - Check if treatment date is in the future
- `ScheduleRequired(key int) bool` - Check if a time slot is scheduled
- `ScheduleRequiredMorning/Noon/Evening() bool` - Convenience methods for time slots
- `ScheduleStatus(key int) nulls.Bool` - Get completion status for a time slot
- `ScheduleStatusMorning/Noon/Evening() nulls.Bool` - Convenience status methods
- `SetAllScheduleRequired(m, n, e bool)` - Set all schedule flags at once
- `SetAllScheduleStatus(m, n, e bool)` - Set all completion flags at once

#### Helper Types

- `TreatmentKey` - Date-based key for organizing treatments (with Past/Current/Future flags)
- `TreatmentStatusStatistics` - Aggregates morning/noon/evening status across treatments
- `TreatmentTemplate` - Used to create series of treatments

#### Constants

```go
Treatement_MORNING = 1
Treatement_NOON    = 2
Treatement_EVENING = 4
```

#### Validation Rules

- `Validate`: Date present, AnimalID present, Drug present, Dosage present, Timebitmap present
- `ValidateCreate`: No additional validation
- `ValidateUpdate`: No additional validation

#### Important Business Logic

- Uses bitmap flags for scheduling (morning=1, noon=2, evening=4)
- `TreatmentsMap()` organizes treatments by date with past/current/future classification
- `TodayStatitics()` aggregates completion status across all treatments for today

---

### VeterinaryVisit

**File:** `models/veterinaryvisit.go`

Records veterinary examinations and diagnoses.

#### Struct Fields

| Field | Type | DB Column | Tags |
|-------|------|-----------|------|
| ID | `uuid.UUID` | `id"` | json:"id" |
| Date | `time.Time` | `date"` | json:"date" |
| UserID | `uuid.UUID` | `user_id"` | json:"user_id" |
| User | `User` | - | belongs_to:"user", json:"user,omitempty" |
| Veterinary | `string` | `veterinary"` | json:"veterinary" |
| AnimalID | `int` | `animal_id"` | json:"animal_id" |
| Animal | `Animal` | - | belongs_to:"animal", json:"-" |
| Diagnostic | `nulls.String` | `diagnostic"` | json:"diagnostic" |
| CreatedAt | `time.Time` | `created_at"` | json:"created_at" |
| UpdatedAt | `time.Time` | `updated_at"` | json:"updated_at" |

#### Relationships

- **belongs_to**: User, Animal

#### Key Methods

- `DateFormated() string` - Returns formatted date

#### Validation Rules

- `Validate`: Date must be present (`TimeIsPresent`)
- `ValidateCreate`: No additional validation
- `ValidateUpdate`: No additional validation

---

### Travel

**File:** `models/travel.go`

Records travel/transportation events for animals.

#### Struct Fields

| Field | Type | DB Column | Tags |
|-------|------|-----------|------|
| ID | `uuid.UUID` | `id"` | json:"id" |
| Date | `time.Time` | `date"` | json:"date" |
| AnimalID | `int` | `animal_id"` | json:"animal_id" |
| Animal | `*Animal` | - | belongs_to:"animal", json:"animal,omitempty" |
| UserID | `uuid.UUID` | `user_id"` | json:"user_id" |
| User | `*User` | - | belongs_to:"user", json:"user" |
| TraveltypeID | `uuid.UUID` | `traveltype_id"` | json:"traveltype_id" |
| Traveltype | `*Traveltype` | - | belongs_to:"traveltype", json:"traveltype,omitempty" |
| TypeDetails | `nulls.String` | `type_details"` | json:"type_details" |
| Distance | `int` | `distance"` | json:"distance" |
| Details | `nulls.String` | `details"` | json:"details" |
| CreatedAt | `time.Time` | `created_at"` | json:"created_at" |
| UpdatedAt | `time.Time` | `updated_at"` | json:"updated_at" |

#### Relationships

- **belongs_to**: Animal, User, Traveltype

#### Key Methods

- `DateFormated() string` - Returns formatted date

#### Validation Rules

- `Validate`: Distance must be present (`IntIsPresent`)
- `ValidateCreate`: No additional validation
- `ValidateUpdate`: No additional validation

---

## Reference Models

### AnimalType

**File:** `models/animaltype.go`

Classification types for animals (e.g., bird, mammal).

#### Struct Fields

| Field | Type | DB Column | Tags |
|-------|------|-----------|------|
| ID | `uuid.UUID` | `id"` | json:"id" |
| Name | `string` | `name"` | json:"name" |
| Default | `bool` | `def"` | json:"default" |
| Description | `nulls.String` | `description"` | json:"description" |
| HasRing | `bool` | `has_ring"` | json:"has_ring" |
| DefaultSpecies | `nulls.String` | `default_species"` | json:"default_species" |
| CreatedAt | `time.Time` | `created_at"` | json:"created_at" |
| UpdatedAt | `time.Time` | `updated_at"` | json:"updated_at" |

#### Key Methods

- `AsMap() map[uuid.UUID]Animaltype` - Converts slice to UUID-indexed map

#### Validation Rules

- `Validate`: Name must be present

---

### AnimalAge

**File:** `models/animalage.go`

Age categories for animals (e.g., juvenile, adult).

#### Struct Fields

| Field | Type | DB Column | Tags |
|-------|------|-----------|------|
| ID | `uuid.UUID` | `id"` | json:"id" |
| Name | `string` | `name"` | json:"name" |
| Description | `nulls.String` | `description"` | json:"description" |
| Default | `bool` | `def"` | json:"def" |
| CreatedAt | `time.Time` | `created_at"` | json:"created_at" |
| UpdatedAt | `time.Time` | `updated_at"` | json:"updated_at" |

#### Validation Rules

- `Validate`: Name must be present

---

### CareType

**File:** `models/caretype.go`

Types of care activities (feeding, cleaning, etc.).

#### Struct Fields

| Field | Type | DB Column | Tags |
|-------|------|-----------|------|
| ID | `uuid.UUID` | `id"` | json:"id" |
| Name | `string` | `name"` | json:"name" |
| Description | `nulls.String` | `description"` | json:"description" |
| Def | `bool` | `def"` | json:"def" |
| Warning | `bool` | `warning"` | json:"warning" |
| ResetWarning | `bool` | `reset_warning"` | json:"reset_warning" |
| Type | `int` | `Type"` | json:"Type" |
| CreatedAt | `time.Time` | `created_at"` | json:"created_at" |
| UpdatedAt | `time.Time` | `updated_at"` | json:"updated_at" |

#### Constants

```go
CareTypeFeed = 1
CareTypeMove = 2
```

#### Validation Rules

- `Validate`: Name must be present

---

### OuttakeType

**File:** `models/outtaketype.go`

Reasons for animal release/departure.

#### Struct Fields

| Field | Type | DB Column | Tags |
|-------|------|-----------|------|
| ID | `uuid.UUID` | `id"` | json:"id" |
| Name | `string` | `name"` | json:"name" |
| Default | `bool` | `def"` | json:"default" |
| Dead | `bool` | `dead"` | json:"dead" |
| Error | `bool` | `error"` | json:"error" |
| Rating | `int` | `rating"` | json:"rating" |
| Description | `nulls.String` | `description"` | json:"description" |
| DiscovererNews | `nulls.String` | `discoverer_news"` | json:"discoverer_news" |
| CreatedAt | `time.Time` | `created_at"` | json:"created_at" |
| UpdatedAt | `time.Time` | `updated_at"` | json:"updated_at" |

#### Validation Rules

- `Validate`: Name must be present

---

### TravelType

**File:** `models/traveltype.go`

Types of travel/transportation.

#### Struct Fields

| Field | Type | DB Column | Tags |
|-------|------|-----------|------|
| ID | `uuid.UUID` | `id"` | json:"id" |
| Name | `string` | `name"` | json:"name" |
| Description | `nulls.String` | `description"` | json:"description" |
| Def | `bool` | `def"` | json:"def" |
| CreatedAt | `time.Time` | `created_at"` | json:"created_at" |
| UpdatedAt | `time.Time` | `updated_at"` | json:"updated_at" |

#### Validation Rules

- `Validate`: Name must be present

---

### Drug

**File:** `models/drug.go`

Medications and drugs available for treatments.

#### Struct Fields

| Field | Type | DB Column | Tags |
|-------|------|-----------|------|
| ID | `uuid.UUID` | `id"` | json:"id" |
| Name | `string` | `name"` | json:"name" |
| Description | `nulls.String` | `description"` | json:"description" |
| Dosages | `[]Dosage` | - | has_many:"dosages", json:"dosages" |
| CreatedAt | `time.Time` | `created_at"` | json:"created_at" |
| UpdatedAt | `time.Time` | `updated_at"` | json:"updated_at" |

#### Relationships

- **has_many**: Dosages

#### Key Methods

- `DosagePerAnimalTypeID() map[uuid.UUID]Dosage` - Maps dosages by animal type UUID

#### Validation Rules

- `Validate`: Name must be present

---

### Dosage

**File:** `models/drug.go` (same file as Drug)

Dosage information for drugs per animal type.

#### Struct Fields

| Field | Type | DB Column | Tags |
|-------|------|-----------|------|
| ID | `uuid.UUID` | `id"` | json:"id" |
| DrugID | `uuid.UUID` | `drug_id"` | json:"drug_id" |
| Drug | `*Drug` | - | belongs_to:"drug", json:"-" |
| AnimaltypeID | `uuid.UUID` | `animaltype_id"` | json:"animaltype_id" |
| Animaltype | `*Animaltype` | - | belongs_to:"animaltype", json:"-" |
| Enabled | `bool` | `enabled"` | json:"enabled" |
| Description | `nulls.String` | `description"` | json:"description" |
| DosagePerGrams | `nulls.Float64` | `dosage_per_grams"` | json:"dosage_per_grams" |
| DosagePerGramsUnit | `nulls.String` | `dosage_per_grams_unit"` | json:"dosage_per_grams_unit" |
| CreatedAt | `time.Time` | `created_at"` | json:"created_at" |
| UpdatedAt | `time.Time` | `updated_at"` | json:"updated_at" |

#### Relationships

- **belongs_to**: Drug, Animaltype

#### Key Methods

- `PerKilo() nulls.Float64` - Converts per-gram dosage to per-kilogram

---

### Species

**File:** `models/species.go`

Taxonomic species reference data.

#### Struct Fields

| Field | Type | DB Column | Tags |
|-------|------|-----------|------|
| ID | `string` | `id"` | json:"id" |
| Species | `string` | `species"` | json:"species" |
| CreavesSpecies | `string` | `creaves_species"` | json:"creaves_species" |
| Class | `string` | `class"` | json:"class" |
| Order | `string` | `order"` | json:"order" |
| Family | `string` | `family"` | json:"family" |
| NativeStatus | `string` | `native_status"` | json:"native_status" |
| AgwGroup | `string` | `agw_group"` | json:"agw_group" |
| SubsideGroup | `string` | `subside_group"` | json:"subside_group" |
| Game | `bool` | `game"` | json:"game" |
| Huntable | `bool` | `huntable"` | json:"huntable" |
| CreatedAt | `time.Time` | `created_at"` | json:"created_at" |
| UpdatedAt | `time.Time` | `updated_at"` | json:"updated_at" |

#### Validation Rules

- `Validate`: Species, Class, Order, Family, CreavesSpecies, SubsideGroup must all be present

---

### Zone

**File:** `models/zone.go`

Physical zones/areas within the care facility.

#### Struct Fields

| Field | Type | DB Column | Tags |
|-------|------|-----------|------|
| ID | `uuid.UUID` | `id"` | json:"id" |
| Zone | `string` | `zone"` | json:"zone" |
| Type | `string` | `type"` | json:"type" |
| Default | `bool` | `default"` | json:"default" |
| CreatedAt | `time.Time` | `created_at"` | json:"created_at" |
| UpdatedAt | `time.Time` | `updated_at"` | json:"updated_at" |

#### Helper Types

- `ZoneKey` - Simple struct with Zone field and `ZoneEscape()` method for URL encoding

#### Validation Rules

- `Validate`: Zone and Type must be present

---

### Locality

**File:** `models/locality.go`

Geographic locality reference data.

#### Struct Fields

| Field | Type | DB Column | Tags |
|-------|------|-----------|------|
| ID | `string` | `id"` | json:"id" |
| Country | `string` | `country"` | json:"country" |
| Region | `string` | `region"` | json:"region" |
| Province | `string` | `province"` | json:"province" |
| Municipality | `string` | `municipality"` | json:"municipality" |
| SubMunicipality | `bool` | `sub_municipality"` | json:"sub_municipality" |
| PostalCode | `string` | `postal_code"` | json:"postal_code" |
| Locality | `string` | `locality"` | json:"locality" |
| Zoning | `string` | `zoning"` | json:"zoning" |
| Direction | `string` | `direction"` | json:"direction" |
| CreatedAt | `time.Time` | `created_at"` | json:"created_at" |
| UpdatedAt | `time.Time` | `updated_at"` | json:"updated_at" |

#### Validation Rules

- `Validate`: ID, Country, Region, Province, Municipality, PostalCode, Locality, Zoning, Direction must all be present

---

### EntryCause

**File:** `models/entry_cause.go`

Reasons for animal entry into care.

#### Struct Fields

| Field | Type | DB Column | Tags |
|-------|------|-----------|------|
| ID | `string` | `id"` | json:"id" |
| Cause | `string` | `cause"` | json:"cause" |
| Detail | `string` | `detail"` | json:"detail" |
| Nature | `string` | `nature"` | json:"nature" |
| Indication | `string` | `indication"` | json:"indication" |
| SortOrder | `int` | `sort_order"` | json:"sort_order" |
| CreatedAt | `time.Time` | `created_at"` | json:"created_at" |
| UpdatedAt | `time.Time` | `updated_at"` | json:"updated_at" |

#### Key Methods

- `Fmt(withId bool) string` - Formats display string, optionally with ID prefix

#### Validation Rules

- `Validate`: ID, Cause, Detail, Nature, Indication must all be present

---

### NativeStatus

**File:** `models/native_status.go`

Native/indigenous status classifications for species.

#### Struct Fields

| Field | Type | DB Column | Tags |
|-------|------|-----------|------|
| ID | `string` | `id"` | json:"id" |
| Status | `string` | `status"` | json:"status" |
| Indication | `string` | `indication"` | json:"indication" |
| Freeable | `bool` | `freeable"` | json:"freeable" |
| Precision | `nulls.String` | `precision"` | json:"precision" |
| CreatedAt | `time.Time` | `created_at"` | json:"created_at" |
| UpdatedAt | `time.Time` | `updated_at"` | json:"updated_at" |

#### Validation Rules

- `Validate`: ID and Status must be present

---

### SubsideGroup

**File:** `models/subside_group.go`

Subsidy/funding groups for financial tracking.

#### Struct Fields

| Field | Type | DB Column | Tags |
|-------|------|-----------|------|
| ID | `string` | `id"` | json:"id" |
| Group | `string` | `group"` | json:"group" |
| Size | `int` | `size"` | json:"size" |
| Amount | `float64` | `amount"` | json:"amount" |
| CreatedAt | `time.Time` | `created_at"` | json:"created_at" |
| UpdatedAt | `time.Time` | `updated_at"` | json:"updated_at" |

#### Validation Rules

- `Validate`: ID, Group, Size must be present

---

### Discoverer

**File:** `models/discoverer.go`

Person who discovered/found the animal.

#### Struct Fields

| Field | Type | DB Column | Tags |
|-------|------|-----------|------|
| ID | `uuid.UUID` | `id"` | json:"id" |
| Firstname | `nulls.String` | `firstname"` | json:"firstname" |
| Lastname | `nulls.String` | `lastname"` | json:"lastname" |
| Address | `nulls.String` | `address"` | json:"address" |
| PostalCode | `nulls.String` | `postal_code"` | json:"postal_code" |
| City | `nulls.String` | `city"` | json:"city" |
| Country | `nulls.String` | `country"` | json:"country" |
| Email | `nulls.String` | `email"` | json:"email" |
| Phone | `nulls.String` | `phone"` | json:"phone" |
| Note | `nulls.String` | `note"` | json:"note" |
| ReturnRequest | `bool` | `return_request"` | json:"return_request" |
| Donation | `nulls.String` | `donation"` | json:"donation" |
| CreatedAt | `time.Time` | `created_at"` | json:"created_at" |
| UpdatedAt | `time.Time` | `updated_at"` | json:"updated_at" |

#### Validation Rules

- `Validate`: Trims string fields (no specific validators)

---

## User & Auth Models

### User

**File:** `models/user.go`

Authentication and user management (generated by buffalo-auth).

#### Struct Fields

| Field | Type | DB Column | Tags |
|-------|------|-----------|------|
| ID | `uuid.UUID` | `id"` | json:"id" |
| CreatedAt | `time.Time` | `created_at"` | json:"created_at" |
| UpdatedAt | `time.Time` | `updated_at"` | json:"updated_at" |
| Login | `string` | `login"` | json:"login" |
| Admin | `bool` | `admin"` | json:"-" |
| Approved | `bool` | `approved"` | json:"-" |
| Shared | `bool` | `shared"` | json:"-" |
| PasswordHash | `string` | `password_hash"` | json:"password_hash" |
| Password | `string` | - | json:"-", db:"-" (transient) |
| PasswordConfirmation | `string` | - | json:"-", db:"-" (transient) |

#### Key Methods

- `SetPasswordHash() error` - Hashes password using bcrypt
- `Create(tx *pop.Connection) (*validate.Errors, error)` - Creates user with password hashing

#### Validation Rules

- `Validate`: Login present, PasswordHash present, Login must be unique
- `ValidateCreate`: Password present, Password must match PasswordConfirmation
- `ValidateUpdate`: Password must match PasswordConfirmation (if changing)

---

### LogEntry

**File:** `models/logentry.go`

Audit log entries for system activity.

#### Struct Fields

| Field | Type | DB Column | Tags |
|-------|------|-----------|------|
| ID | `uuid.UUID` | `id"` | json:"id" |
| User | `User` | - | belongs_to:"user", json:"user" |
| UserID | `uuid.UUID` | `user_id"` | json:"user_id" |
| Description | `string` | `description"` | json:"description" |
| CreatedAt | `time.Time` | `created_at"` | json:"created_at" |
| UpdatedAt | `time.Time` | `updated_at"` | json:"updated_at" |

#### Relationships

- **belongs_to**: User

#### Key Methods

- `CreatedAtFormated() string` - Returns formatted creation date
- `UpdatedAtFormated() string` - Returns formatted update date

#### Validation Rules

- `Validate`: Description must be present

---

## Utility Files

### models.go

**File:** `models/models.go`

Database connection initialization.

#### Global Variables

- `DB *pop.Connection` - Global database connection

#### Functions

- `init()` - Initializes DB connection using `GO_ENV` environment variable (defaults to "development")
- Enables Pop debug mode in development environment

---

### constants.go

**File:** `models/constants.go`

Date/time formatting constants and timezone utilities.

#### Constants

```go
DateTimeFormat = "2006/01/02 15:04"
DateFormat     = "2006/01/02"
```

#### Functions

- `NowOffset() time.Time` - Returns current time adjusted for timezone offset (works around Pop UTC bug)
- `init()` - Overrides Pop's `NowFunc` to use `NowOffset()`

---

## Relationship Summary

```
Animal
├── belongs_to: Animalage
├── belongs_to: Animaltype
├── belongs_to: Discovery
│   └── belongs_to: EntryCause
│   └── belongs_to: Discoverer
├── belongs_to: Intake
├── belongs_to: Outtake (optional)
│   └── belongs_to: Outtaketype
├── has_many: Cares
│   └── belongs_to: Caretype
├── has_many: Treatments
├── has_many: VetVisits
│   └── belongs_to: User

Travel
├── belongs_to: Animal
├── belongs_to: User
└── belongs_to: Traveltype

Drug
└── has_many: Dosages
    ├── belongs_to: Drug
    └── belongs_to: Animaltype

Logentry
└── belongs_to: User
```

## Common Patterns

### Validation Pattern
All models follow the same validation interface:
- `Validate(tx *pop.Connection) (*validate.Errors, error)` - General validation
- `ValidateCreate(tx *pop.Connection) (*validate.Errors, error)` - Creation-specific validation
- `ValidateUpdate(tx *pop.Connection) (*validate.Errors, error)` - Update-specific validation

### String Serialization
Most models implement:
- `String() string` - JSON marshaling for single entity
- Slice type `String() string` - JSON marshaling for collections

### Date Formatting
Core models with date fields implement:
- `DateFormated() string` - Uses `DateTimeFormat` constant

### Bitmap Pattern
Treatment model uses bitmap flags for scheduling:
- `Timebitmap` - Required schedule slots (morning/noon/evening)
- `Timedonebitmap` - Completed slots
- Bit values: Morning=1, Noon=2, Evening=4
