package actions

import (
	"net/http"
	"time"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/x/responder"
)

// duplicateSubmissionWindow is the time frame during which a second,
// byte-identical Create is considered an accidental double submission
// (double-click, double Enter key, browser refresh re-POSTing the form)
// rather than a legitimate new record. See GitHub issue #100: duplicate
// entries were reported by users in production on Chrome.
const duplicateSubmissionWindow = 2 * time.Minute

// recentDuplicateExists reports whether at least one record of the given model
// matching the fingerprint query was created within the duplicate submission
// window.
//
// The fingerprint query receives the bound values as positional parameters and
// must end comparing created_at (the ? placeholder at the end is bound to the
// window cutoff). Nullable columns must be compared with MySQL's null-safe
// equality operator `<=>` so that NULL values match each other.
//
// The check is best-effort: a query error is logged and reported as "no
// duplicate" so the guard can never block a legitimate creation because of a
// transient database problem.
func recentDuplicateExists(logger buffalo.Logger, tx *pop.Connection, model interface{}, fingerprintQuery string, args ...interface{}) bool {
	allArgs := append(args, time.Now().Add(-duplicateSubmissionWindow))
	exists, err := tx.Where(fingerprintQuery+" AND created_at > ?", allArgs...).Exists(model)
	if err != nil {
		logger.Errorf("duplicate submission guard query failed: %v", err)
		return false
	}
	return exists
}

// treatmentFingerprintQuery returns the fingerprint comparison for a treatment:
// same animal, date, drug, dosage, remarks and morning/noon/evening bitmap.
const treatmentFingerprintQuery = "animal_id = ? AND date = ? AND drug <=> ? AND dosage <=> ? AND remarks <=> ? AND timebitmap = ? AND timedonebitmap = ?"

// veterinaryvisitFingerprintQuery returns the fingerprint comparison for a
// veterinary visit: same animal, date, veterinary and diagnostic.
const veterinaryvisitFingerprintQuery = "animal_id = ? AND date = ? AND veterinary <=> ? AND diagnostic <=> ?"

// careFingerprintQuery returns the fingerprint comparison for a care entry:
// same animal, date, care type, weight, note, flags and link.
const careFingerprintQuery = "animal_id = ? AND date = ? AND type_id = ? AND weight <=> ? AND note <=> ? AND clean <=> ? AND in_warning <=> ? AND link_to_id <=> ? AND heat_source <=> ? AND oxygen = ?"

// intakeFingerprintQuery returns the fingerprint comparison for an intake
// (reception flow): same date, general state, wounds and parasites details
// and remarks. A re-POSTed reception form (double-click, refresh, back
// button) carries byte-identical intake values, while two genuinely distinct
// intakes differ in at least one of them (issue #199: 4 identical animals
// created by repeated submissions).
const intakeFingerprintQuery = "date = ? AND general <=> ? AND has_wounds = ? AND wounds <=> ? AND has_parasites = ? AND parasites <=> ? AND remarks <=> ?"

// duplicateSubmissionRedirect answers a detected double submission: a warning
// flash plus a redirect to the "back" parameter when present, else the given
// redirect target, so the user lands back on the animal without a duplicate
// having been created.
func duplicateSubmissionRedirect(c buffalo.Context, i18nKey, redirectFormat string, redirectArgs ...interface{}) error {
	return responder.Wants("html", func(c buffalo.Context) error {
		c.Flash().Add("warning", T.Translate(c, i18nKey))
		if back := safeBackParam(c); back != "/" {
			return c.Redirect(http.StatusSeeOther, back)
		}
		return c.Redirect(http.StatusSeeOther, redirectFormat, redirectArgs...)
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusConflict, r.JSON(map[string]string{"error": T.Translate(c, i18nKey)}))
	}).Respond(c)
}
