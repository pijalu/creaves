package actions

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"os"
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
// Issue #34: animal attachments — upload, serve, delete with auth and
// type/size whitelists.
// ---------------------------------------------------------------------------

// attTStorageRoot points the storage at a temp dir for the test and restores
// the real limits afterwards.
func attTStorageRoot(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("ATTACHMENTS_DIR", dir)
	return dir
}

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

func TestAttachmentsUploadServeDelete34(t *testing.T) {
	tx := searchTestDB(t)
	fx := createAnimalSearchFixtures(t, tx)
	root := attTStorageRoot(t)

	login, password := feedingGuideUser(t, true)
	client, baseURL := feedingGuideLogin(t, login, password)

	// ---- upload a PNG
	resp := attTUpload(t, client, baseURL, fx.animalC, "file", "hé llo photo.png", "image/png", attTPng())
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusSeeOther, resp.StatusCode, "body: %s", body)

	a := attTFind(t, tx, fx.animalC)
	t.Cleanup(func() {
		models.DB.RawQuery("DELETE FROM attachments WHERE animal_id = ?", fx.animalC).Exec()
		os.RemoveAll(filepath.Join(root, "animal-"+strconv.Itoa(fx.animalC)))
	})

	require.Equal(t, "image/png", a.ContentType)
	require.Equal(t, models.AttachmentKindImage, a.Kind)
	require.Equal(t, int64(len(attTPng())), a.Size)
	require.True(t, a.UploadedBy.Valid, "uploader recorded")
	// original name sanitized but preserved for display
	require.Equal(t, "hé llo photo.png", a.Filename)
	// stored under a generated uuid path inside the animal dir
	require.Contains(t, a.StoragePath, "animal-"+strconv.Itoa(fx.animalC)+"/")

	stored := filepath.Join(root, filepath.FromSlash(a.StoragePath))
	fi, err := os.Stat(stored)
	require.NoError(t, err, "file stored on disk")
	require.Equal(t, a.Size, fi.Size())

	// ---- serve it back
	resp2, err := client.Get(baseURL + "/attachments/" + a.ID.String())
	require.NoError(t, err)
	body2, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	require.Equal(t, http.StatusOK, resp2.StatusCode)
	require.Equal(t, "image/png", resp2.Header.Get("Content-Type"))
	require.True(t, strings.Contains(resp2.Header.Get("Content-Disposition"), "photo.png"))
	require.Equal(t, attTPng(), body2, "bytes must survive the roundtrip")

	// ---- owner delete removes row + file
	req, err := http.NewRequest("POST", baseURL+"/attachments/"+a.ID.String()+"/delete", nil)
	require.NoError(t, err)
	resp3, err := client.Do(req)
	require.NoError(t, err)
	resp3.Body.Close()
	require.Equal(t, http.StatusSeeOther, resp3.StatusCode)

	cnt, err := tx.Where("animal_id = ?", fx.animalC).Count(&models.Attachment{})
	require.NoError(t, err)
	require.Equal(t, 0, cnt, "row deleted")
	_, err = os.Stat(stored)
	require.True(t, os.IsNotExist(err), "file removed from disk")
}

func TestAttachmentsRejectsTypeAndSize34(t *testing.T) {
	tx := searchTestDB(t)
	fx := createAnimalSearchFixtures(t, tx)
	attTStorageRoot(t)

	// shrink the image limit so the oversize case stays cheap
	old := models.AttachmentMaxImageSize
	models.AttachmentMaxImageSize = 64
	t.Cleanup(func() { models.AttachmentMaxImageSize = old })

	login, password := feedingGuideUser(t, true)
	client, baseURL := feedingGuideLogin(t, login, password)
	t.Cleanup(func() {
		models.DB.RawQuery("DELETE FROM attachments WHERE animal_id = ?", fx.animalC).Exec()
	})

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
	root := attTStorageRoot(t)

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
	t.Cleanup(func() {
		models.DB.RawQuery("DELETE FROM attachments WHERE animal_id = ?", fx.animalC).Exec()
		os.RemoveAll(filepath.Join(root, "animal-"+strconv.Itoa(fx.animalC)))
	})
	// put a real file at the storage location
	abs := filepath.Join(root, filepath.FromSlash(a.StoragePath))
	require.NoError(t, os.MkdirAll(filepath.Dir(abs), 0o755))
	require.NoError(t, os.WriteFile(abs, attTPng(), 0o644))

	// ---- a different non-admin user is forbidden
	otherClient, otherBase := feedingGuideLogin(t, other, "fgpass123")
	req, err := http.NewRequest("POST", otherBase+"/attachments/"+a.ID.String()+"/delete", nil)
	require.NoError(t, err)
	resp, err := otherClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusForbidden, resp.StatusCode)

	// ---- the owner may delete
	ownerClient, ownerBase := feedingGuideLogin(t, owner, "fgpass123")
	req, err = http.NewRequest("POST", ownerBase+"/attachments/"+a.ID.String()+"/delete", nil)
	require.NoError(t, err)
	resp, err = ownerClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusSeeOther, resp.StatusCode)

	cnt, err := tx.Where("id = ?", a.ID).Count(&models.Attachment{})
	require.NoError(t, err)
	require.Equal(t, 0, cnt)
	_, err = os.Stat(abs)
	require.True(t, os.IsNotExist(err), "file removed")
}

// ownerUUID34 resolves the uuid of a login created by feedingGuideUser.
func ownerUUID34(t *testing.T, login string) uuid.UUID {
	t.Helper()
	u := &models.User{}
	require.NoError(t, models.DB.Where("login = ?", login).First(u))
	return u.ID
}
