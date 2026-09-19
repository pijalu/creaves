package actions

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"creaves/models"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/buffalo/binding"
	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
)

// ---------------------------------------------------------------------------
// Animal attachments: photos and videos stored on the local disk, metadata in
// the attachments table (issue #34). One storage root per instance (each
// deployment is per-center), overridable with ATTACHMENTS_DIR.
// ---------------------------------------------------------------------------

// attachmentsRoot returns the directory holding uploaded attachment files.
func attachmentsRoot() string {
	if d := strings.TrimSpace(os.Getenv("ATTACHMENTS_DIR")); d != "" {
		return d
	}
	return filepath.Join(".", "attachments-storage")
}

// attachmentExtension picks a canonical file extension for the stored copy.
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

// AttachmentsServe streams an attachment file inline (images render in the
// gallery, mp4/webm play in the browser). Range requests are supported via
// http.ServeContent, which is required for video seeking.
func AttachmentsServe(c buffalo.Context) error {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}
	a, err := loadAttachment(tx, c)
	if err != nil {
		return err
	}

	path := filepath.Join(attachmentsRoot(), filepath.FromSlash(a.StoragePath))
	f, err := os.Open(path)
	if err != nil {
		return c.Error(http.StatusNotFound, err)
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	disposition := fmt.Sprintf("inline; filename=%q", a.Filename)
	c.Response().Header().Set("Content-Disposition", disposition)
	c.Response().Header().Set("Content-Type", a.ContentType)
	http.ServeContent(c.Response(), c.Request(), a.Filename, fi.ModTime(), f)
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

	animalID, err := strconv.Atoi(c.Param("animal_id"))
	if err != nil || animalID == 0 {
		return c.Error(http.StatusNotFound, fmt.Errorf("animal not found"))
	}
	animal := &models.Animal{}
	if err := tx.Find(animal, animalID); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	redir := attachmentsRedirectURL(c, animalID)

	f, ferr := c.File("file")
	if ferr != nil || !f.Valid() {
		c.Flash().Add("danger", T.Translate(c, "attachments.upload.missing"))
		return c.Redirect(http.StatusSeeOther, redir)
	}

	a, ok := buildAttachmentFromUpload(c, f, user.ID, animalID)
	if !ok {
		return c.Redirect(http.StatusSeeOther, redir)
	}

	rel := filepath.Join("animal-"+strconv.Itoa(animalID), a.ID.String()+attachmentExtension(a.ContentType))
	if err := storeAttachmentFile(f.File, rel, models.AttachmentMaxSize(a.ContentType)); err != nil {
		c.Logger().Errorf("attachments: store failed: %v", err)
		c.Flash().Add("danger", T.Translate(c, "attachments.upload.failed"))
		return c.Redirect(http.StatusSeeOther, redir)
	}
	a.StoragePath = filepath.ToSlash(rel)

	if verrs, err := tx.ValidateAndCreate(a); err != nil || verrs.HasAny() {
		_ = os.Remove(filepath.Join(attachmentsRoot(), rel))
		c.Logger().Errorf("attachments: db create failed: %v %v", verrs, err)
		c.Flash().Add("danger", T.Translate(c, "attachments.upload.failed"))
		return c.Redirect(http.StatusSeeOther, redir)
	}

	auditAnimalChange(c, tx, animalID, models.AuditEntityAttachment, auditEntityID(a.ID), models.AuditActionCreate, nil, a)
	c.Flash().Add("success", T.Translate(c, "attachments.created.success"))
	return c.Redirect(http.StatusSeeOther, redir)
}

// buildAttachmentFromUpload validates the upload (presence, type whitelist,
// size limit) and returns the metadata record; ok=false means the upload was
// rejected and the danger flash already queued.
func buildAttachmentFromUpload(c buffalo.Context, f binding.File, uploaderID uuid.UUID, animalID int) (*models.Attachment, bool) {
	ct, kind := sniffAttachmentType(f)
	if kind == "" {
		c.Logger().Warnf("attachments: rejected type %q for %q", ct, f.Filename)
		c.Flash().Add("danger", T.Translate(c, "attachments.upload.type"))
		return nil, false
	}
	max := models.AttachmentMaxSize(ct)
	if f.Size <= 0 || f.Size > max {
		c.Logger().Warnf("attachments: rejected size %d for %q", f.Size, f.Filename)
		c.Flash().Add("danger", T.Translate(c, "attachments.upload.size"))
		return nil, false
	}
	return &models.Attachment{
		ID:          uuid.Must(uuid.NewV4()),
		AnimalID:    animalID,
		Filename:    models.SanitizeFilename(f.Filename),
		ContentType: ct,
		Size:        f.Size,
		Kind:        kind,
		UploadedBy:  nulls.NewUUID(uploaderID),
	}, true
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

// storeAttachmentFile copies the upload to its disk location under the
// storage root, never accepting more than max bytes.
func storeAttachmentFile(src io.Reader, rel string, max int64) error {
	path := filepath.Join(attachmentsRoot(), rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	dst, err := os.Create(path)
	if err != nil {
		return err
	}
	defer dst.Close()
	n, err := io.Copy(dst, io.LimitReader(src, max+1))
	if err != nil {
		_ = os.Remove(path)
		return err
	}
	if n > max {
		_ = os.Remove(path)
		return fmt.Errorf("upload exceeds %d bytes", max)
	}
	return nil
}

// AttachmentsDestroy removes an attachment (file + row). Only its uploader
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
	if a.StoragePath != "" {
		if err := os.Remove(filepath.Join(attachmentsRoot(), filepath.FromSlash(a.StoragePath))); err != nil && !os.IsNotExist(err) {
			c.Logger().Errorf("attachments: could not remove file %q: %v", a.StoragePath, err)
		}
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
