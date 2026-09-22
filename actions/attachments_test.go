package actions

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"creaves/models"

	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Issue #34 (reopened): animal attachments — binary content stored in the
// database (attachment_blobs), upload/serve/delete with auth and type/size
// whitelists. No files are written to disk.
// ---------------------------------------------------------------------------

// attTPng returns a minimal valid PNG (1x1 pixel).
func attTPng() []byte {
	return []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
		0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4, 0x89, 0x00, 0x00, 0x00,
		0x0d, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x62, 0x00, 0x01, 0x00, 0x00,
		0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00, 0x00, 0x00, 0x00, 0x49,
		0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
	}
}

// attTUpload posts a multipart file upload and returns the response.
func attTUpload(t *testing.T, client *http.Client, baseURL string, animalID int, fieldname, filename, contentType string, body []byte) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", filename)
	require.NoError(t, err)
	_, err = fw.Write(body)
	require.NoError(t, err)
	require.NoError(t, w.Close())

	req, err := http.NewRequest("POST", baseURL+"/animals/"+strconv.Itoa(animalID)+"/attachments", &buf)
	require.NoError(t, err)
	req.Header.Set("Content-Type", w.FormDataContentType())
	setCSRFHeader(t, client, baseURL, req)
	resp, err := client.Do(req)
	require.NoError(t, err)
	return resp
}

// attTFind locates the single attachment row for an animal.
func attTFind(t *testing.T, tx *pop.Connection, animalID int) *models.Attachment {
	t.Helper()
	a := &models.Attachment{}
	require.NoError(t, tx.Where("animal_id = ?", animalID).Order("created_at desc").First(a), "attachment row")
	return a
}

// attTBlobCount counts the blob rows of an attachment.
func attTBlobCount(t *testing.T, tx *pop.Connection, id uuid.UUID) int {
	t.Helper()
	cnt, err := tx.Where("attachment_id = ?", id).Count(&models.AttachmentBlob{})
	require.NoError(t, err)
	return cnt
}

// attTCleanup removes attachments + blobs of an animal after the test.
func attTCleanup(t *testing.T, animalID int) {
	t.Cleanup(func() {
		models.DB.RawQuery("DELETE FROM attachment_blobs WHERE attachment_id IN (SELECT id FROM attachments WHERE animal_id = ?)", animalID).Exec()
		models.DB.RawQuery("DELETE FROM attachments WHERE animal_id = ?", animalID).Exec()
	})
}

func TestAttachmentsUploadServeDelete34(t *testing.T) {
	tx := searchTestDB(t)
	fx := createAnimalSearchFixtures(t, tx)

	login, password := feedingGuideUser(t, true)
	client, baseURL := feedingGuideLogin(t, login, password)

	// ---- upload a PNG
	resp := attTUpload(t, client, baseURL, fx.animalC, "file", "hé llo photo.png", "image/png", attTPng())
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusSeeOther, resp.StatusCode, "body: %s", body)

	a := attTFind(t, tx, fx.animalC)
	attTCleanup(t, fx.animalC)

	require.Equal(t, "image/png", a.ContentType)
	require.Equal(t, models.AttachmentKindImage, a.Kind)
	require.Equal(t, int64(len(attTPng())), a.Size)
	require.True(t, a.UploadedBy.Valid, "uploader recorded")
	// original name sanitized but preserved for display
	require.Equal(t, "hé llo photo.png", a.Filename)
	// storage_path is a logical locator only — nothing on disk
	require.Contains(t, a.StoragePath, "animal-"+strconv.Itoa(fx.animalC)+"/")
	require.NoFileExists(t, filepath.Join("attachments-storage", filepath.FromSlash(a.StoragePath)))

	// ---- binary stored in the database, byte-identical
	data, found, err := models.LoadAttachmentBlob(tx, a.ID)
	require.NoError(t, err)
	require.True(t, found, "blob row must exist after upload")
	require.Equal(t, attTPng(), data)

	// ---- serve it back
	resp2, err := client.Get(baseURL + "/attachments/" + a.ID.String())
	require.NoError(t, err)
	body2, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	require.Equal(t, http.StatusOK, resp2.StatusCode)
	require.Equal(t, "image/png", resp2.Header.Get("Content-Type"))
	require.True(t, strings.Contains(resp2.Header.Get("Content-Disposition"), "photo.png"))
	require.Equal(t, attTPng(), body2, "bytes must survive the roundtrip")

	// ---- owner delete removes row + blob
	req, err := http.NewRequest("POST", baseURL+"/attachments/"+a.ID.String()+"/delete", nil)
	require.NoError(t, err)
	setCSRFHeader(t, client, baseURL, req)
	resp3, err := client.Do(req)
	require.NoError(t, err)
	resp3.Body.Close()
	require.Equal(t, http.StatusSeeOther, resp3.StatusCode)

	cnt, err := tx.Where("animal_id = ?", fx.animalC).Count(&models.Attachment{})
	require.NoError(t, err)
	require.Equal(t, 0, cnt, "row deleted")
	require.Equal(t, 0, attTBlobCount(t, tx, a.ID), "blob deleted")
}

func TestAttachmentsRejectsTypeAndSize34(t *testing.T) {
	tx := searchTestDB(t)
	fx := createAnimalSearchFixtures(t, tx)

	// shrink the image limit so the oversize case stays cheap
	old := models.AttachmentMaxImageSize
	models.AttachmentMaxImageSize = 64
	t.Cleanup(func() { models.AttachmentMaxImageSize = old })

	login, password := feedingGuideUser(t, true)
	client, baseURL := feedingGuideLogin(t, login, password)
	attTCleanup(t, fx.animalC)

	// ---- wrong type (plain text) rejected
	resp := attTUpload(t, client, baseURL, fx.animalC, "file", "note.txt", "text/plain", []byte("not an image"))
	resp.Body.Close()
	require.Equal(t, http.StatusSeeOther, resp.StatusCode)

	cnt, err := tx.Where("animal_id = ?", fx.animalC).Count(&models.Attachment{})
	require.NoError(t, err)
	require.Equal(t, 0, cnt, "no row for rejected type")

	// ---- oversize image rejected
	resp = attTUpload(t, client, baseURL, fx.animalC, "file", "big.png", "image/png", bytes.Repeat([]byte{0x89, 0x50}, 200))
	resp.Body.Close()
	require.Equal(t, http.StatusSeeOther, resp.StatusCode)

	cnt, err = tx.Where("animal_id = ?", fx.animalC).Count(&models.Attachment{})
	require.NoError(t, err)
	require.Equal(t, 0, cnt, "no row for oversize upload")
}

func TestAttachmentsDeleteOwnership34(t *testing.T) {
	tx := searchTestDB(t)
	fx := createAnimalSearchFixtures(t, tx)

	owner, _ := feedingGuideUser(t, false)
	other, _ := feedingGuideUser(t, false)
	ownerID := ownerUUID34(t, owner)

	a := &models.Attachment{
		ID:          uuid.Must(uuid.NewV4()),
		AnimalID:    fx.animalC,
		Filename:    "x.png",
		ContentType: "image/png",
		Size:        int64(len(attTPng())),
		StoragePath: "animal-" + strconv.Itoa(fx.animalC) + "/" + uuid.Must(uuid.NewV4()).String() + ".png",
		Kind:        models.AttachmentKindImage,
		UploadedBy:  nulls.NewUUID(ownerID),
	}
	require.NoError(t, tx.Create(a))
	require.NoError(t, models.StoreAttachmentBlob(tx, a.ID, attTPng()))
	attTCleanup(t, fx.animalC)

	// ---- a different non-admin user is forbidden
	otherClient, otherBase := feedingGuideLogin(t, other, "fgpass123")
	req, err := http.NewRequest("POST", otherBase+"/attachments/"+a.ID.String()+"/delete", nil)
	require.NoError(t, err)
	setCSRFHeader(t, otherClient, otherBase, req)
	resp, err := otherClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusForbidden, resp.StatusCode)

	// ---- the owner may delete
	ownerClient, ownerBase := feedingGuideLogin(t, owner, "fgpass123")
	req, err = http.NewRequest("POST", ownerBase+"/attachments/"+a.ID.String()+"/delete", nil)
	require.NoError(t, err)
	setCSRFHeader(t, ownerClient, ownerBase, req)
	resp, err = ownerClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusSeeOther, resp.StatusCode)

	cnt, err := tx.Where("id = ?", a.ID).Count(&models.Attachment{})
	require.NoError(t, err)
	require.Equal(t, 0, cnt)
	require.Equal(t, 0, attTBlobCount(t, tx, a.ID), "blob removed with the row")
}

// TestAttachmentsServeMissingBlob404 pins the DB-only contract: an
// attachment row without a blob serves 404 (there is no disk fallback).
func TestAttachmentsServeMissingBlob404(t *testing.T) {
	tx := searchTestDB(t)
	fx := createAnimalSearchFixtures(t, tx)
	attTCleanup(t, fx.animalC)

	login, password := feedingGuideUser(t, true)
	client, baseURL := feedingGuideLogin(t, login, password)

	a := &models.Attachment{
		ID:          uuid.Must(uuid.NewV4()),
		AnimalID:    fx.animalC,
		Filename:    "x.png",
		ContentType: "image/png",
		Size:        int64(len(attTPng())),
		StoragePath: "animal-" + strconv.Itoa(fx.animalC) + "/" + uuid.Must(uuid.NewV4()).String() + ".png",
		Kind:        models.AttachmentKindImage,
	}
	require.NoError(t, tx.Create(a))

	resp, err := client.Get(baseURL + "/attachments/" + a.ID.String())
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// TestAttachmentBlobStoreLoadDelete34 exercises the storage helpers
// directly, including the idempotent re-store (upsert).
func TestAttachmentBlobStoreLoadDelete34(t *testing.T) {
	tx := searchTestDB(t)
	fx := createAnimalSearchFixtures(t, tx)

	a := &models.Attachment{
		ID:          uuid.Must(uuid.NewV4()),
		AnimalID:    fx.animalC,
		Filename:    "blob.png",
		ContentType: "image/png",
		Size:        int64(len(attTPng())),
		StoragePath: "animal-" + strconv.Itoa(fx.animalC) + "/" + uuid.Must(uuid.NewV4()).String() + ".png",
		Kind:        models.AttachmentKindImage,
	}
	require.NoError(t, tx.Create(a))
	attTCleanup(t, fx.animalC)

	// not found before store
	_, found, err := models.LoadAttachmentBlob(tx, a.ID)
	require.NoError(t, err)
	require.False(t, found)

	// store + load back
	require.NoError(t, models.StoreAttachmentBlob(tx, a.ID, attTPng()))
	data, found, err := models.LoadAttachmentBlob(tx, a.ID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, attTPng(), data)

	// re-store replaces (upsert, still one row)
	other := []byte{0xde, 0xad, 0xbe, 0xef}
	require.NoError(t, models.StoreAttachmentBlob(tx, a.ID, other))
	require.Equal(t, 1, attTBlobCount(t, tx, a.ID))
	data, found, err = models.LoadAttachmentBlob(tx, a.ID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, other, data)

	// delete
	require.NoError(t, models.DeleteAttachmentBlob(tx, a.ID))
	require.Equal(t, 0, attTBlobCount(t, tx, a.ID))
	_, found, err = models.LoadAttachmentBlob(tx, a.ID)
	require.NoError(t, err)
	require.False(t, found)
}

// ownerUUID34 resolves the uuid of a login created by feedingGuideUser.
func ownerUUID34(t *testing.T, login string) uuid.UUID {
	t.Helper()
	u := &models.User{}
	require.NoError(t, models.DB.Where("login = ?", login).First(u))
	return u.ID
}

// TestAttachmentsCreateOuttakenAnimalAuthR1 pins BUG-R1: once an animal has
// an outtake, only admins may upload attachments (server-side; the hidden
// form in the template is not a security boundary).
func TestAttachmentsCreateOuttakenAnimalAuthR1(t *testing.T) {
	tx := searchTestDB(t)
	fx := createAnimalSearchFixtures(t, tx)
	attTCleanup(t, fx.animalA)
	attTCleanup(t, fx.animalC)

	countFor := func(animalID int) int {
		t.Helper()
		cnt, err := tx.Where("animal_id = ?", animalID).Count(&models.Attachment{})
		require.NoError(t, err)
		return cnt
	}

	// ---- non-admin on outtaken animal (animalA has an outtake) → blocked
	plainLogin, _ := feedingGuideUser(t, false)
	plainClient, plainBase := feedingGuideLogin(t, plainLogin, "fgpass123")
	resp := attTUpload(t, plainClient, plainBase, fx.animalA, "file", "x.png", "image/png", attTPng())
	resp.Body.Close()
	require.Equal(t, http.StatusSeeOther, resp.StatusCode)
	rows := []models.Attachment{}
	require.NoError(t, tx.Where("animal_id = ?", fx.animalA).All(&rows))
	for _, r := range rows {
		t.Logf("DBG row: id=%s animal=%d file=%s uploader=%v", r.ID, r.AnimalID, r.Filename, r.UploadedBy)
	}
	require.Equal(t, 0, countFor(fx.animalA), "non-admin upload on outtaken animal must be rejected")

	// ---- admin on outtaken animal → allowed
	adminLogin, _ := feedingGuideUser(t, true)
	adminClient, adminBase := feedingGuideLogin(t, adminLogin, "fgpass123")
	resp = attTUpload(t, adminClient, adminBase, fx.animalA, "file", "x.png", "image/png", attTPng())
	resp.Body.Close()
	require.Equal(t, http.StatusSeeOther, resp.StatusCode)
	require.Equal(t, 1, countFor(fx.animalA), "admin upload on outtaken animal must succeed")

	// ---- non-admin on in-care animal (animalC, no outtake) → allowed
	resp = attTUpload(t, plainClient, plainBase, fx.animalC, "file", "y.png", "image/png", attTPng())
	resp.Body.Close()
	require.Equal(t, http.StatusSeeOther, resp.StatusCode)
	require.Equal(t, 1, countFor(fx.animalC), "non-admin upload on in-care animal must succeed")
}

// TestAnimalDestroyRemovesAttachmentsR2 pins BUG-R2: destroying an animal
// (admin, error-outtake flow) must remove its attachment rows AND blobs —
// orphaned media must not stay servable.
func TestAnimalDestroyRemovesAttachmentsR2(t *testing.T) {
	tx := searchTestDB(t)
	fx := createAnimalSearchFixtures(t, tx)
	attTCleanup(t, fx.animalC)

	adminLogin, _ := feedingGuideUser(t, true)
	adminClient, adminBase := feedingGuideLogin(t, adminLogin, "fgpass123")

	// upload an attachment on the in-care animal
	resp := attTUpload(t, adminClient, adminBase, fx.animalC, "file", "gone.png", "image/png", attTPng())
	resp.Body.Close()
	require.Equal(t, http.StatusSeeOther, resp.StatusCode)
	a := attTFind(t, tx, fx.animalC)
	require.Equal(t, 1, attTBlobCount(t, tx, a.ID))

	// destroy the animal (DELETE /animals/{id})
	// The destroy flow creates an extra error-outtake referencing the animal;
	// remove it first (LIFO: runs before the fixture cleanup) so the fixture
	// can delete the animal row without FK violations.
	t.Cleanup(func() {
		tx.RawQuery("DELETE FROM outtakes WHERE animal_id = ?", fx.animalC).Exec()
	})
	req, err := http.NewRequest("DELETE", adminBase+"/animals/"+strconv.Itoa(fx.animalC), nil)
	require.NoError(t, err)
	setCSRFHeader(t, adminClient, adminBase, req)
	resp2, err := adminClient.Do(req)
	require.NoError(t, err)
	resp2.Body.Close()
	require.Equal(t, http.StatusSeeOther, resp2.StatusCode)

	cnt, err := tx.Where("animal_id = ?", fx.animalC).Count(&models.Attachment{})
	require.NoError(t, err)
	require.Equal(t, 0, cnt, "attachment rows removed on animal destroy")
	require.Equal(t, 0, attTBlobCount(t, tx, a.ID), "attachment blobs removed on animal destroy")

	// the media URL is gone too
	resp3, err := adminClient.Get(adminBase + "/attachments/" + a.ID.String())
	require.NoError(t, err)
	resp3.Body.Close()
	require.Equal(t, http.StatusNotFound, resp3.StatusCode)
}

// TestAttachmentsRedirectBackParamSafeR5 pins BUG-R5: a hostile back= param
// must not be honored — redirects stay on local paths.
func TestAttachmentsRedirectBackParamSafeR5(t *testing.T) {
	tx := searchTestDB(t)
	fx := createAnimalSearchFixtures(t, tx)
	attTCleanup(t, fx.animalC)

	adminLogin, _ := feedingGuideUser(t, true)
	adminClient, adminBase := feedingGuideLogin(t, adminLogin, "fgpass123")

	for _, evil := range []string{"https://evil.example/phish", "//evil.example/phish"} {
		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		fw, err := w.CreateFormFile("file", "x.png")
		require.NoError(t, err)
		_, err = fw.Write(attTPng())
		require.NoError(t, err)
		require.NoError(t, w.Close())

		req, err := http.NewRequest("POST", adminBase+"/animals/"+strconv.Itoa(fx.animalC)+"/attachments?back="+url.QueryEscape(evil), &buf)
		require.NoError(t, err)
		req.Header.Set("Content-Type", w.FormDataContentType())
		setCSRFHeader(t, adminClient, adminBase, req)
		resp, err := adminClient.Do(req)
		require.NoError(t, err)
		resp.Body.Close()
		require.Equal(t, http.StatusSeeOther, resp.StatusCode)
		loc := resp.Header.Get("Location")
		require.True(t, strings.HasPrefix(loc, "/") && !strings.HasPrefix(loc, "//"),
			"back=%q must not redirect off-site, got Location %q", evil, loc)
	}
}
