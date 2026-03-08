# Creaves Project - TODO, Optimizations, Refactors & Cleanups

This document lists all identified improvements for the Creaves Buffalo web application. Items are organized by category and priority to facilitate systematic implementation by development teams or AI agents.

---

## Table of Contents

1. [Critical Issues](#critical-issues)
2. [Security Improvements](#security-improvements)
3. [Performance Optimizations](#performance-optimizations)
4. [Code Quality & Refactoring](#code-quality--refactoring)
5. [Dependency Updates](#dependency-updates)
6. [Testing Improvements](#testing-improvements)
7. [Frontend Modernization](#frontend-modernization)
8. [Database & Migrations](#database--migrations)
9. [Documentation](#documentation)
10. [DevOps & Deployment](#devops--deployment)

---

## Critical Issues

### 1.1 Fix Incorrect Hash Function Name
**Priority:** HIGH  
**Location:** `actions/helper.go:13`  
**Issue:** Function named `sha256` but actually uses SHA-1 algorithm  
**Impact:** Security misrepresentation, weaker hash than advertised  

**Current Code:**
```go
func sha256(s string) string {
	h := sha1.New()  // <-- Using SHA-1, not SHA-256
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}
```

**Required Action:**
- Rename function to `sha1` or change implementation to use `sha256.New()`
- Update all callers if renamed
- Consider if cryptographic strength is needed for use case (feeding zone hashing)

---

### 1.2 Remove Debug Logging Statements
**Priority:** MEDIUM  
**Location:** Multiple files (83+ occurrences)  
**Issue:** Production code contains excessive debug logging  

**Files Affected:**
- `actions/animals.go` - 12+ debug statements
- `actions/cares.go` - 8+ debug statements
- `actions/suggestions.go` - 8+ debug statements
- `actions/users.go` - 6+ debug statements
- `actions/treatments.go` - 6+ debug statements
- `export/export.go`, `excel/excel.go` - Multiple debug statements

**Required Action:**
- Replace `c.Logger().Debugf()` with proper logging levels
- Remove commented debug statements (e.g., `//c.Logger().Debugf(...)`)
- Consider implementing structured logging

---

### 1.3 Fix Cache Implementation
**Priority:** MEDIUM  
**Location:** `actions/cache_utils.go`  
**Issue:** Cache refresh goroutine calls function that requires Buffalo context  

**Current Code:**
```go
go func() {
    for {
        time.Sleep(cacheUpdateInterval)
        refreshWeightLossCache()  // <-- Can't work without context
    }
}()
```

**Required Action:**
- Implement proper background cache refresh with DB connection
- Or remove auto-refresh and rely on manual invalidation
- Add cache metrics/monitoring

---

### 1.4 Remove Test/Debug Directories
**Priority:** MEDIUM  
**Location:** `/stuff`, `/excel/testdir`, `/drugs`  
**Issue:** Development/test code in production codebase  

**Directories to Review:**
- `/stuff/` - Contains test code (`stuff.go`, `feeding/feedi.go`)
- `/excel/testdir/` - Test utilities
- `/drugs/gen.go` - Drug generation script (may be needed for seeding)

**Required Action:**
- Move useful utilities to `/cmd/` or `/scripts/`
- Remove or archive unnecessary test files
- Document purpose of remaining files

---

## Security Improvements

### 2.1 Enable SSL Redirect in Production
**Priority:** HIGH  
**Location:** `actions/app.go:49`  
**Issue:** SSL redirect is commented out  

**Current Code:**
```go
// Automatically redirect to SSL
// app.Use(forceSSL())
```

**Required Action:**
- Uncomment for production environments
- Configure proper SSL certificates
- Update Docker configuration for SSL termination

---

### 2.2 Update bcrypt Cost Factor
**Priority:** MEDIUM  
**Location:** `models/user.go:31`  
**Issue:** Using `bcrypt.DefaultCost` (currently 10, considered weak in 2024)  

**Required Action:**
- Increase to `bcrypt.MinCost` + 2 or custom value (12-14)
- Implement password rehashing on login for existing users
- Document password policy

---

### 2.3 Add Rate Limiting
**Priority:** MEDIUM  
**Location:** `actions/app.go`  
**Issue:** No rate limiting on authentication endpoints  

**Required Action:**
- Add rate limiting middleware to `/auth/*` routes
- Implement account lockout after failed attempts
- Add CAPTCHA after N failed attempts

---

### 2.4 Implement Content Security Policy
**Priority:** MEDIUM  
**Location:** `actions/app.go:199` (forceSSL function)  
**Issue:** No CSP headers configured  

**Required Action:**
- Configure secure headers in `forceSSL()` function
- Add CSP, X-Content-Type-Options, X-Frame-Options
- Test with browser dev tools

---

### 2.5 Review SQL Injection Prevention
**Priority:** HIGH  
**Location:** Multiple files  
**Issue:** Some queries use string formatting instead of parameterized queries  

**Examples:**
```go
// actions/animals.go:156
query := fmt.Sprintf("animal_id IN (%s)", strings.Join(animalIds, ","))

// actions/suggestions.go:28
qroot := "SELECT DISTINCT " + field + " FROM " + table
```

**Required Action:**
- Audit all raw SQL queries
- Replace with parameterized queries where possible
- Validate table/field names against whitelist

---

## Performance Optimizations

### 3.1 Optimize N+1 Queries
**Priority:** HIGH  
**Location:** `actions/animals.go`  
**Issue:** Multiple functions load related data inefficiently  

**Current State:**
- `EnrichAnimals()` - Better bulk loading implemented
- `EnrichAnimalsOptimized()` - Good implementation exists
- `EnrichAnimal()` - Still uses individual queries

**Required Action:**
- Apply bulk loading pattern to `EnrichAnimal()`
- Review all resource Show actions for N+1 issues
- Add query logging/monitoring to identify bottlenecks

---

### 3.2 Add Database Indexes
**Priority:** HIGH  
**Location:** Database schema  
**Issue:** Missing indexes on frequently queried columns  

**Recommended Indexes:**
```sql
-- Animals table
CREATE INDEX idx_animals_outtake_id ON animals(outtake_id);
CREATE INDEX idx_animals_zone ON animals(zone);
CREATE INDEX idx_animals_cage ON animals(cage);
CREATE INDEX idx_animals_intake_date ON animals(IntakeDate);

-- Cares table
CREATE INDEX idx_cares_animal_id_date ON cares(animal_id, date);
CREATE INDEX idx_cares_type_id ON cares(type_id);

-- Treatments table
CREATE INDEX idx_treatments_animal_id_date ON treatments(animal_id, date);
CREATE INDEX idx_treatments_date ON treatments(date);

-- Discoveries table
CREATE INDEX idx_discoveries_location ON discoveries(location);
```

**Required Action:**
- Create migration for new indexes
- Test query performance before/after
- Document index usage

---

### 3.3 Implement Query Caching
**Priority:** MEDIUM  
**Location:** Dashboard, Feeding pages  
**Issue:** Complex queries run on every request  

**Candidates for Caching:**
- `listAnimalCountPerType()` - Dashboard statistics
- `listAnimalWithWeightLoss()` - Already cached (12h)
- Feeding calculations - Zone-based caching

**Required Action:**
- Extend cache pattern to other expensive queries
- Add cache invalidation on data changes
- Consider Redis for shared cache in multi-instance deployments

---

### 3.4 Optimize Template Rendering
**Priority:** LOW  
**Location:** `actions/render.go`  
**Issue:** Templates loaded from embedded FS on every request  

**Required Action:**
- Verify template caching is enabled
- Pre-compile templates in production
- Consider CDN for static assets

---

### 3.5 Add Pagination to Heavy Listings
**Priority:** MEDIUM  
**Location:** Resource List actions  
**Issue:** Some listings may load all records  

**Required Action:**
- Review all List() implementations
- Ensure pagination is enforced
- Add "per_page" limits (max 100)

---

## Code Quality & Refactoring

### 4.1 Standardize Error Handling
**Priority:** HIGH  
**Location:** Throughout codebase  
**Issue:** Inconsistent error handling patterns  

**Current Patterns Found:**
```go
// Pattern 1: Return error
if err != nil {
    return err
}

// Pattern 2: Flash and redirect
if err != nil {
    c.Flash().Add("danger", err.Error())
    return c.Redirect(...)
}

// Pattern 3: Custom error page
if err != nil {
    return c.Error(http.StatusNotFound, err)
}
```

**Required Action:**
- Create error handling utility functions
- Standardize on consistent pattern per operation type
- Add proper error context/wrapping
- Implement centralized error pages

---

### 4.2 Extract Common Middleware
**Priority:** MEDIUM  
**Location:** `actions/users.go:50-80`  
**Issue:** Authorization logic duplicated across resources  

**Current Pattern:**
```go
cu := GetCurrentUser(c)
if !cu.Admin {
    return c.Error(http.StatusForbidden, fmt.Errorf("Admin rights required"))
}
```

**Required Action:**
- Create `AdminOnly()` middleware
- Create `OwnerOrAdmin(resourceID string)` middleware
- Apply middleware to resource groups instead of individual actions

---

### 4.3 Refactor Large Functions
**Priority:** MEDIUM  
**Location:** Multiple files  
**Issue:** Functions exceeding 100+ lines  

**Functions to Refactor:**
- `actions/animals.go:EnrichAnimals()` - 120 lines
- `actions/animals.go:Update()` - 150+ lines
- `actions/animals.go:Create()` - 100+ lines
- `actions/feeding.go:calculateFeeding()` - 60 lines (complex logic)

**Required Action:**
- Extract helper functions
- Apply single responsibility principle
- Add function documentation

---

### 4.4 Implement Service Layer
**Priority:** MEDIUM  
**Location:** Business logic in actions  
**Issue:** Business logic mixed with HTTP handling  

**Current State:** Actions contain:
- HTTP request handling
- Business logic (feeding calculations, animal enrichment)
- Database operations
- Template rendering

**Required Action:**
- Create `/services/` package
- Move business logic to service functions
- Keep actions thin (HTTP only)
- Example: `services/AnimalService.EnrichAnimals()`

---

### 4.5 Fix Magic Numbers and Strings
**Priority:** LOW  
**Location:** Throughout codebase  
**Issue:** Hard-coded values without explanation  

**Examples:**
```go
// actions/feeding.go
const HIGHTIMELIMIT = 2 * time.Hour  // Good
const NEARTIMELIMIT = 15 * time.Minute  // Good

// actions/dashboard_weightloss.go
AND t1.weight_in_grams <= t2.weight_in_grams * 0.93 // Magic number: 7% threshold
```

**Required Action:**
- Extract to named constants
- Add comments explaining business rationale
- Consider making configurable

---

### 4.6 Improve Variable Naming
**Priority:** LOW  
**Location:** Multiple files  
**Issue:** Unclear variable names  

**Examples:**
```go
cu  // Current user - OK but could be clearer
ct  // Care types - ambiguous
ot  // Outtake type - ambiguous
dts // Discoveries - unclear
```

**Required Action:**
- Rename to descriptive names
- Apply consistent naming conventions
- Document abbreviations if kept

---

### 4.7 Remove Unused Code
**Priority:** LOW  
**Location:** Throughout codebase  
**Issue:** Dead code and commented sections  

**Examples:**
- `actions/animals.go:254` - Commented debug log
- `actions/typehelper.go:39` - Commented debug logs
- `templates/home/delete_me.txt` - Placeholder file

**Required Action:**
- Remove commented code (use git history if needed)
- Delete unused template files
- Clean up unused imports

---

### 4.8 Add Input Validation
**Priority:** MEDIUM  
**Location:** Form handlers  
**Issue:** Limited client/server-side validation  

**Required Action:**
- Add validation rules to models
- Implement client-side validation
- Add server-side validation feedback
- Sanitize all user inputs

---

## Dependency Updates

### 5.1 Update Go Version
**Priority:** HIGH  
**Location:** `go.mod:3`  
**Current:** Go 1.18 (EOL)  
**Target:** Go 1.21+ (LTS)  

**Required Action:**
- Update `go.mod` to `go 1.21`
- Test all functionality
- Update CI/CD pipelines
- Update Dockerfile base image

---

### 5.2 Update Buffalo Framework
**Priority:** HIGH  
**Location:** `go.mod:6`  
**Current:** v0.18.9 (2022)  
**Target:** Latest stable  

**Required Action:**
- Review Buffalo changelog for breaking changes
- Update incrementally (v0.18 → v0.19 → latest)
- Test all routes and functionality
- Update middleware if needed

---

### 5.3 Update Pop (ORM)
**Priority:** HIGH  
**Location:** `go.mod:7`  
**Current:** v6.1.0  
**Target:** Latest v6.x or v7.x  

**Required Action:**
- Check for migration guide
- Update and test all queries
- Review deprecated methods

---

### 5.4 Update Frontend Dependencies
**Priority:** MEDIUM  
**Location:** `package.json`  
**Issues:**
- Bootstrap 4.6.2 (Bootstrap 5 is current)
- jQuery 3.6.0 (3.7+ available)
- Webpack 5.65.0 (5.89+ available)
- Many dev dependencies outdated

**Required Action:**
- Audit dependencies: `npm audit`
- Update Bootstrap to 5.x (requires CSS/JS updates)
- Update Webpack and plugins
- Consider migrating to Vite for faster builds

---

### 5.5 Update golang.org/x Packages
**Priority:** MEDIUM  
**Location:** `go.mod`  
**Current:** Various 2023 versions  
**Issue:** Security updates available  

**Required Action:**
- Run `go get -u golang.org/x/...`
- Test crypto functions
- Update import paths if needed

---

### 5.6 Remove Unused Dependencies
**Priority:** LOW  
**Location:** `go.mod`  
**Issue:** Dependencies that may not be needed  

**Candidates:**
- `github.com/gobuffalo/tags` (v2 and v3 both present)
- `github.com/gobuffalo/validate` (v2 and v3 both present)
- `gopkg.in/yaml.v2` (v3 available)

**Required Action:**
- Run `go mod tidy`
- Verify all imports are needed
- Remove duplicates

---

## Testing Improvements

### 6.1 Add Comprehensive Test Coverage
**Priority:** HIGH  
**Current State:** Only 2 test files exist  
- `actions/feeding_test.go` - Feeding calculation tests
- `stuff/feeding/feed_test.go` - Test utilities  

**Required Action:**
- Add tests for all resource actions (CRUD)
- Add model validation tests
- Add middleware tests
- Add integration tests for critical paths
- Target: 70%+ code coverage

---

### 6.2 Add Integration Tests
**Priority:** MEDIUM  
**Location:** New test files needed  
**Issue:** No end-to-end testing  

**Required Action:**
- Set up test database
- Create integration test suite
- Test authentication flow
- Test animal CRUD operations
- Test dashboard calculations

---

### 6.3 Add API Tests
**Priority:** MEDIUM  
**Location:** Resource actions  
**Issue:** JSON/XML responses not tested  

**Required Action:**
- Test all API endpoints
- Validate response schemas
- Test error responses
- Add performance benchmarks

---

### 6.4 Implement Test Fixtures
**Priority:** MEDIUM  
**Location:** `/fixtures`  
**Current:** Only `sample.toml` exists  

**Required Action:**
- Create test data fixtures
- Add factory functions for common models
- Seed test database consistently

---

### 6.5 Add Frontend Tests
**Priority:** LOW  
**Location:** `/assets/js`  
**Issue:** No JavaScript testing  

**Required Action:**
- Add Jest or similar
- Test critical JS functions
- Test form validations
- Test autocomplete features

---

## Frontend Modernization

### 7.1 Update Bootstrap Version
**Priority:** MEDIUM  
**Location:** `package.json`, templates  
**Current:** Bootstrap 4.6.2  
**Target:** Bootstrap 5.3+  

**Required Action:**
- Update npm package
- Replace jQuery-dependent components
- Update template classes (`.custom-control` → `.form-check`)
- Test all forms and modals

---

### 7.2 Reduce jQuery Dependency
**Priority:** LOW  
**Location:** `assets/js/application.js`  
**Issue:** Heavy reliance on jQuery  

**Required Action:**
- Replace jQuery-UJS with native fetch
- Update autocomplete to vanilla JS or modern library
- Consider Alpine.js for interactivity

---

### 7.3 Improve Responsive Design
**Priority:** MEDIUM  
**Location:** Templates, SCSS  
**Issue:** Mobile experience not verified  

**Required Action:**
- Test all pages on mobile devices
- Add responsive tables
- Improve touch targets
- Add mobile navigation

---

### 7.4 Add Loading Indicators
**Priority:** LOW  
**Location:** Forms, AJAX operations  
**Issue:** No feedback on long operations  

**Required Action:**
- Add spinners to form submissions
- Add progress indicators for bulk operations
- Add toast notifications for async operations

---

### 7.5 Implement Dark Mode
**Priority:** LOW  
**Location:** SCSS, templates  
**Issue:** No theme options  

**Required Action:**
- Add CSS variables for theming
- Implement dark mode toggle
- Persist preference in localStorage
- Respect system preference

---

### 7.6 Improve Form UX
**Priority:** MEDIUM  
**Location:** All form templates  
**Issue:** Basic form implementations  

**Required Action:**
- Add field-level validation messages
- Implement auto-save for long forms
- Add confirmation dialogs for destructive actions
- Improve error display

---

## Database & Migrations

### 8.1 Review Schema Design
**Priority:** MEDIUM  
**Location:** Migrations, Models  
**Issue:** Schema evolved organically  

**Required Action:**
- Document entity relationships
- Identify normalization issues
- Plan schema improvements
- Create migration strategy

---

### 8.2 Add Foreign Key Constraints
**Priority:** MEDIUM  
**Location:** Database schema  
**Issue:** Some relationships lack FK constraints  

**Required Action:**
- Audit all relationships
- Add missing FK constraints
- Test cascade behaviors
- Document constraints

---

### 8.3 Implement Soft Deletes
**Priority:** LOW  
**Location:** Models  
**Issue:** Records permanently deleted  

**Required Action:**
- Add `deleted_at` column to critical tables
- Update queries to filter soft-deleted records
- Add admin interface for managing deleted records

---

### 8.4 Add Audit Trail
**Priority:** MEDIUM  
**Location:** Critical tables  
**Issue:** No change tracking  

**Required Action:**
- Add audit columns (`created_by`, `updated_by`)
- Consider audit log table for sensitive changes
- Implement in middleware

---

### 8.5 Optimize Trigger Usage
**Priority:** LOW  
**Location:** `migrations/trigger-animal.sql`  
**Issue:** Triggers may impact performance  

**Required Action:**
- Review trigger logic
- Consider application-level alternatives
- Document trigger behavior

---

## Documentation

### 9.1 Update README
**Priority:** HIGH  
**Location:** `README.md`  
**Current:** Minimal description  

**Required Action:**
- Add project overview
- Document setup instructions
- Add development workflow
- Include deployment guide
- Add troubleshooting section

---

### 9.2 Add API Documentation
**Priority:** MEDIUM  
**Location:** New file needed  
**Issue:** No API docs  

**Required Action:**
- Document all endpoints
- Add request/response examples
- Include authentication requirements
- Consider OpenAPI/Swagger spec

---

### 9.3 Document Business Logic
**Priority:** MEDIUM  
**Location:** Code comments, Wiki  
**Issue:** Complex logic undocumented  

**Topics to Document:**
- Feeding calculation algorithm
- Animal lifecycle workflow
- Treatment scheduling
- Weight loss detection logic

---

### 9.4 Add Architecture Documentation
**Priority:** LOW  
**Location:** New file needed  
**Issue:** No architecture overview  

**Required Action:**
- Create ARCHITECTURE.md
- Document component relationships
- Add deployment diagram
- Document data flow

---

### 9.5 Create Developer Onboarding Guide
**Priority:** MEDIUM  
**Location:** New file needed  
**Issue:** Setup process not documented  

**Required Action:**
- Document prerequisites
- Step-by-step setup guide
- Common development tasks
- Testing instructions

---

## DevOps & Deployment

### 10.1 Update Docker Configuration
**Priority:** MEDIUM  
**Location:** `Dockerfile`  
**Issues:**
- Multi-stage build can be optimized
- Base images may be outdated
- No health checks

**Required Action:**
- Use official Go images
- Add health check endpoint
- Optimize layer caching
- Add non-root user

---

### 10.2 Add Health Check Endpoint
**Priority:** MEDIUM  
**Location:** `actions/app.go`  
**Issue:** No health check for monitoring  

**Required Action:**
- Add `/health` endpoint
- Check database connectivity
- Return service status
- Add readiness probe

---

### 10.3 Implement Environment Configuration
**Priority:** MEDIUM  
**Location:** `database.yml`, `config/`  
**Issue:** Limited environment support  

**Required Action:**
- Support multiple environments
- Use environment variables for secrets
- Add configuration validation
- Document required env vars

---

### 10.4 Add CI/CD Pipeline
**Priority:** MEDIUM  
**Location:** New files needed  
**Issue:** No automated testing/deployment  

**Required Action:**
- Set up GitHub Actions or similar
- Add automated tests
- Add linting (golangci-lint)
- Add automated builds
- Add deployment automation

---

### 10.5 Add Monitoring & Logging
**Priority:** MEDIUM  
**Location:** Throughout application  
**Issue:** Limited observability  

**Required Action:**
- Add structured logging
- Implement request tracing
- Add metrics collection
- Set up error tracking (Sentry)

---

### 10.6 Optimize Build Process
**Priority:** LOW  
**Location:** `build.sh`, `webpack.config.js`  
**Issue:** Build process can be improved  

**Required Action:**
- Add build versioning
- Implement incremental builds
- Add build artifacts validation
- Document build process

---

## Quick Wins (Low Effort, High Impact)

1. **Remove debug logging** - Clean up 80+ debug statements
2. **Fix sha256 function name** - Rename or fix implementation
3. **Add database indexes** - Create migration for performance
4. **Update Go version** - Move to supported LTS version
5. **Add health check** - Simple endpoint for monitoring
6. **Update README** - Basic documentation improvements
7. **Run go mod tidy** - Clean up dependencies
8. **Remove dead code** - Delete unused files and comments
9. **Add error handling middleware** - Centralize error pages
10. **Enable SSL in production** - Uncomment forceSSL

---

## Implementation Priority

### Phase 1: Critical (Week 1-2)
- Fix security issues (SSL, SQL injection)
- Update Go and major dependencies
- Add critical database indexes
- Remove debug logging

### Phase 2: High Priority (Week 3-4)
- Refactor large functions
- Add comprehensive testing
- Implement service layer
- Add monitoring/health checks

### Phase 3: Medium Priority (Month 2)
- Frontend modernization
- Performance optimizations
- Documentation improvements
- CI/CD pipeline

### Phase 4: Long-term (Month 3+)
- Architecture improvements
- Advanced features
- Technical debt reduction
- Continuous improvements

---

## Notes for AI Agents

When working on items from this TODO:

1. **Always run tests** after making changes
2. **Check for existing patterns** before implementing new ones
3. **Make incremental changes** - small, testable commits
4. **Update this document** when completing items
5. **Ask for clarification** if requirements are unclear
6. **Consider backward compatibility** for database changes
7. **Test with production-like data** volumes
8. **Document all changes** in commit messages

---

## Last Updated

2025-02-25

## Generated By

Automated code analysis of Creaves Buffalo application
