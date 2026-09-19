package actions

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"creaves/models"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/buffalo/binding"
	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
)

// ---------------------------------------------------------------------------
// Animal attachments: photos and videos. Metadata lives in the attachments
// table, binary content in the database (attachment_blobs, issue #34
// reopened). The database is the only storage: no files are written to disk.
// storage_path stays populated as a logical locator for the audit trail.
// ---------------------------------------------------------------------------

// attachmentExtension picks a canonical file extension for the storage path.
func attachmentExtension(contentType string) string {
	switch models.AttachmentKind(contentType) {
	case models.AttachmentKindVideo:
		return ".mp4"
	default:
		return ".jpg"
	}
}

// loadAttachment finds an attachment by the {attachment_id} route param.
func loadAttachment(tx *pop.Connection, c buffalo.Context) (*models.Attachment, error) {
	a := &models.Attachment{}
	if err := tx.Find(a, c.Param("attachment_id")); err != nil {
		return nil, c.Error(http.StatusNotFound, err)
	}
	return a, nil
}

// AttachmentsServe streams an attachment inline (images render in the
// gallery, mp4/webm play in the browser). Range requests are supported via
// http.ServeContent, which is required for video seeking. The binary comes
// from the database.
func AttachmentsServe(c buffalo.Context) error {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}
	a, err := loadAttachment(tx, c)
	if err != nil {
		return err
	}

	data, found, err := models.LoadAttachmentBlob(tx, a.ID)
	if err != nil {
		return err
	}
	if !found {
		return c.Error(http.StatusNotFound, fmt.Errorf("attachment binary not found"))
	}

	disposition := fmt.Sprintf("inline; filename=%q", a.Filename)
	c.Response().Header().Set("Content-Disposition", disposition)
	c.Response().Header().Set("Content-Type", a.ContentType)
	http.ServeContent(c.Response(), c.Request(), a.Filename, time.Time{}, bytes.NewReader(data))
	return nil
}

// AttachmentsCreate handles the multipart upload for an animal.
// Any authenticated user may upload on an animal still in care (admins on
// outtaken animals, matching the care edit rules).
func AttachmentsCreate(c buffalo.Context) error {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}
	user := GetCurrentUser(c)
	if user == nil {
		return c.Error(http.StatusUnauthorized, fmt.Errorf("not authenticated"))
	}

	animalID, _ := strconv.Atoi(c.Param("animal_id"))
	animal := &models.Animal{}
	if animalID == 0 || tx.Find(animal, animalID) != nil {
		return c.Error(http.StatusNotFound, fmt.Errorf("animal not found"))
	}

	redir := attachmentsRedirectURL(c, animalID)

	// Server-side enforcement of the rule stated in the doc comment: once the
	// animal has an outtake, only admins may add attachments (the template
	// hides the form, but that is not a security boundary).
	if rejectOuttakenAnimalUpload(c, animal, user, redir) {
		return nil // redirect + flash already queued by the helper
	}

	f, ferr := c.File("file")
	if ferr != nil || !f.Valid() {
		c.Flash().Add("danger", T.Translate(c, "attachments.upload.missing"))
		return c.Redirect(http.StatusSeeOther, redir)
	}

	a, data, ok := buildAttachmentFromUpload(c, f, user.ID, animalID)
	if !ok {
		return c.Redirect(http.StatusSeeOther, redir)
	}

	rel := filepath.Join("animal-"+strconv.Itoa(animalID), a.ID.String()+attachmentExtension(a.ContentType))
	a.StoragePath = filepath.ToSlash(rel)

	if verrs, err := tx.ValidateAndCreate(a); err != nil || verrs.HasAny() {
		c.Logger().Errorf("attachments: db create failed: %v %v", verrs, err)
		c.Flash().Add("danger", T.Translate(c, "attachments.upload.failed"))
		return c.Redirect(http.StatusSeeOther, redir)
	}
	// Same transaction as the row insert: a blob failure rolls back both.
	if err := models.StoreAttachmentBlob(tx, a.ID, data); err != nil {
		c.Logger().Errorf("attachments: blob store failed (is max_allowed_packet >= upload size?): %v", err)
		return err
	}

	auditAnimalChange(c, tx, animalID, models.AuditEntityAttachment, auditEntityID(a.ID), models.AuditActionCreate, nil, a)
	c.Flash().Add("success", T.Translate(c, "attachments.created.success"))
	return c.Redirect(http.StatusSeeOther, redir)
}

// rejectOuttakenAnimalUpload enforces the "in-care only for non-admins"
// upload rule. Returns true when the upload was rejected (danger flash queued
// and redirect written — note buffalo's c.Redirect returns nil, so the caller
// must stop via the bool).
func rejectOuttakenAnimalUpload(c buffalo.Context, animal *models.Animal, user *models.User, redir string) bool {
	if animal.OuttakeID.Valid && !user.Admin {
		c.Flash().Add("danger", T.Translate(c, "attachments.upload.outtaken"))
		_ = c.Redirect(http.StatusSeeOther, redir)
		return true
	}
	return false
}

// buildAttachmentFromUpload validates the upload (presence, type whitelist,
// size limit), reads the bytes (never more than the limit) and returns the
// metadata record + content; ok=false means the upload was rejected and the
// danger flash already queued.
func buildAttachmentFromUpload(c buffalo.Context, f binding.File, uploaderID uuid.UUID, animalID int) (*models.Attachment, []byte, bool) {
	ct, kind := sniffAttachmentType(f)
	if kind == "" {
		c.Logger().Warnf("attachments: rejected type %q for %q", ct, f.Filename)
		c.Flash().Add("danger", T.Translate(c, "attachments.upload.type"))
		return nil, nil, false
	}
	max := models.AttachmentMaxSize(ct)
	if f.Size <= 0 || f.Size > max {
		c.Logger().Warnf("attachments: rejected size %d for %q", f.Size, f.Filename)
		c.Flash().Add("danger", T.Translate(c, "attachments.upload.size"))
		return nil, nil, false
	}
	data, ok := readUploadLimited(f.File, max)
	if !ok {
		c.Logger().Errorf("attachments: read of upload %q failed or exceeded limit", f.Filename)
		c.Flash().Add("danger", T.Translate(c, "attachments.upload.failed"))
		return nil, nil, false
	}
	return &models.Attachment{
		ID:          uuid.Must(uuid.NewV4()),
		AnimalID:    animalID,
		Filename:    models.SanitizeFilename(f.Filename),
		ContentType: ct,
		Size:        f.Size,
		Kind:        kind,
		UploadedBy:  nulls.NewUUID(uploaderID),
	}, data, true
}

// sniffAttachmentType determines the attachment MIME type from the first
// bytes, falling back to the declared multipart header type.
func sniffAttachmentType(f binding.File) (contentType, kind string) {
	head := make([]byte, 512)
	n, _ := f.File.Read(head)
	if n > 0 {
		sniffed := http.DetectContentType(head[:n])
		if models.AttachmentKind(sniffed) != "" {
			if _, err := f.File.Seek(0, io.SeekStart); err == nil {
				return sniffed, models.AttachmentKind(sniffed)
			}
		}
	}
	if _, err := f.File.Seek(0, io.SeekStart); err != nil {
		return "", ""
	}
	declared := f.FileHeader.Header.Get("Content-Type")
	return declared, models.AttachmentKind(declared)
}

// readUploadLimited reads the whole upload (already rewound to the start by
// the type sniffing), never accepting more than max bytes.
func readUploadLimited(src io.Reader, max int64) ([]byte, bool) {
	data, err := io.ReadAll(io.LimitReader(src, max+1))
	if err != nil || int64(len(data)) > max || len(data) == 0 {
		return nil, false
	}
	return data, true
}

// AttachmentsDestroy removes an attachment (blob + row). Only its uploader
// or an admin may delete it.
func AttachmentsDestroy(c buffalo.Context) error {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}
	user := GetCurrentUser(c)
	if user == nil {
		return c.Error(http.StatusUnauthorized, fmt.Errorf("not authenticated"))
	}
	a, err := loadAttachment(tx, c)
	if err != nil {
		return err
	}
	isOwner := a.UploadedBy.Valid && a.UploadedBy.UUID == user.ID
	if !isOwner && !user.Admin {
		return c.Error(http.StatusForbidden, fmt.Errorf("restricted"))
	}

	if err := tx.Destroy(a); err != nil {
		return err
	}
	// Same transaction as the row destroy; failure rolls both back. (The FK
	// also cascades, this is belt and braces for installs without it.)
	if err := models.DeleteAttachmentBlob(tx, a.ID); err != nil {
		c.Logger().Errorf("attachments: could not delete blob %s: %v", a.ID, err)
		return err
	}

	auditAnimalChange(c, tx, a.AnimalID, models.AuditEntityAttachment, auditEntityID(a.ID), models.AuditActionDelete, a, nil)
	c.Flash().Add("success", T.Translate(c, "attachments.destroyed.success"))
	return c.Redirect(http.StatusSeeOther, attachmentsRedirectURL(c, a.AnimalID))
}

// attachmentsRedirectURL builds the post-action redirect (honors back=).
func attachmentsRedirectURL(c buffalo.Context, animalID int) string {
	if back := c.Param("back"); back != "" {
		return back
	}
	return "/animals/" + strconv.Itoa(animalID) + "#nav-media"
}

// setAnimalAttachments loads the gallery for the animal show page.
func setAnimalAttachments(tx *pop.Connection, c buffalo.Context, animalID int) {
	atts := &models.Attachments{}
	if err := tx.Where("animal_id = ?", animalID).Order("created_at desc").All(atts); err != nil {
		c.Logger().Errorf("attachments: gallery load failed for animal %d: %v", animalID, err)
		return
	}
	c.Set("animalAttachments", atts)
}
