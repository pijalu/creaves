package actions

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"creaves/models"

	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Bug 2026-10-27 #7 (second review batch): media upload gains a
// comment/details field — set at upload time, editable afterwards by the
// uploader or an admin, shown in the gallery.
// ---------------------------------------------------------------------------

// attCommentUpload posts a multipart upload carrying an optional
// "comment" form field alongside the file.
func attCommentUpload(t *testing.T, client *http.Client, baseURL string, animalID int, filename, comment string) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", filename)
	require.NoError(t, err)
	_, err = fw.Write(attTPng())
	require.NoError(t, err)
	if comment != "" {
		require.NoError(t, w.WriteField("comment", comment))
	}
	require.NoError(t, w.Close())

	req, err := http.NewRequest("POST", baseURL+"/animals/"+strconv.Itoa(animalID)+"/attachments", &buf)
	require.NoError(t, err)
	req.Header.Set("Content-Type", w.FormDataContentType())
	setCSRFHeader(t, client, baseURL, req)
	resp, err := client.Do(req)
	require.NoError(t, err)
	return resp
}

func TestAttachmentCommentSetAtUploadAndEditable(t *testing.T) {
	tx := searchTestDB(t)
	fx := createAnimalSearchFixtures(t, tx)
	attTCleanup(t, fx.animalC)

	login, password := feedingGuideUser(t, true)
	client, baseURL := feedingGuideLogin(t, login, password)

	// ---- upload with a comment: the comment lands on the row
	resp := attCommentUpload(t, client, baseURL, fx.animalC, "wound.png", "left flank, day 3")
	resp.Body.Close()
	require.Equal(t, http.StatusSeeOther, resp.StatusCode)

	a := attTFind(t, tx, fx.animalC)
	require.True(t, a.Comment.Valid, "comment recorded at upload time")
	require.Equal(t, "left flank, day 3", a.Comment.String)

	// ---- owner edits the comment afterwards
	req, err := http.NewRequest("POST", baseURL+"/attachments/"+a.ID.String()+"/comment",
		strings.NewReader(url.Values{"comment": {"healing well, day 7"}}.Encode()))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	setCSRFHeader(t, client, baseURL, req)
	resp2, err := client.Do(req)
	require.NoError(t, err)
	resp2.Body.Close()
	require.Equal(t, http.StatusSeeOther, resp2.StatusCode)

	updated := &models.Attachment{}
	require.NoError(t, tx.Find(updated, a.ID))
	require.Equal(t, "healing well, day 7", updated.Comment.String, "comment editable after upload")
}

func TestAttachmentCommentOwnershipAndCap(t *testing.T) {
	tx := searchTestDB(t)
	fx := createAnimalSearchFixtures(t, tx)
	attTCleanup(t, fx.animalC)

	ownerLogin, ownerPassword := feedingGuideUser(t, false)
	ownerClient, ownerBase := feedingGuideLogin(t, ownerLogin, ownerPassword)

	otherLogin, otherPassword := feedingGuideUser(t, false)
	otherClient, otherBase := feedingGuideLogin(t, otherLogin, otherPassword)

	resp := attCommentUpload(t, ownerClient, ownerBase, fx.animalC, "a.png", "original")
	resp.Body.Close()
	require.Equal(t, http.StatusSeeOther, resp.StatusCode)
	a := attTFind(t, tx, fx.animalC)

	// ---- a non-owner non-admin may not edit the comment
	req, err := http.NewRequest("POST", otherBase+"/attachments/"+a.ID.String()+"/comment",
		strings.NewReader(url.Values{"comment": {"hijacked"}}.Encode()))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	setCSRFHeader(t, otherClient, otherBase, req)
	resp2, err := otherClient.Do(req)
	require.NoError(t, err)
	resp2.Body.Close()
	require.Equal(t, http.StatusForbidden, resp2.StatusCode)

	unchanged := &models.Attachment{}
	require.NoError(t, tx.Find(unchanged, a.ID))
	require.Equal(t, "original", unchanged.Comment.String, "non-owner edit rejected")

	// ---- the cap holds: a too-long comment is rejected with a flash
	long := strings.Repeat("x", 501)
	req3, err := http.NewRequest("POST", ownerBase+"/attachments/"+a.ID.String()+"/comment",
		strings.NewReader(url.Values{"comment": {long}}.Encode()))
	require.NoError(t, err)
	req3.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	setCSRFHeader(t, ownerClient, ownerBase, req3)
	resp3, err := ownerClient.Do(req3)
	require.NoError(t, err)
	body, _ := io.ReadAll(resp3.Body)
	resp3.Body.Close()
	require.Equal(t, http.StatusSeeOther, resp3.StatusCode, "rejected with a flash, not a crash: %s", body)

	final := &models.Attachment{}
	require.NoError(t, tx.Find(final, a.ID))
	require.Equal(t, "original", final.Comment.String, "too-long comment rejected")
}

func TestAttachmentCommentBlankClears(t *testing.T) {
	tx := searchTestDB(t)
	fx := createAnimalSearchFixtures(t, tx)
	attTCleanup(t, fx.animalC)

	login, password := feedingGuideUser(t, true)
	client, baseURL := feedingGuideLogin(t, login, password)

	resp := attCommentUpload(t, client, baseURL, fx.animalC, "b.png", "to clear")
	resp.Body.Close()
	a := attTFind(t, tx, fx.animalC)

	req, err := http.NewRequest("POST", baseURL+"/attachments/"+a.ID.String()+"/comment",
		strings.NewReader(url.Values{"comment": {"   "}}.Encode()))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	setCSRFHeader(t, client, baseURL, req)
	resp2, err := client.Do(req)
	require.NoError(t, err)
	resp2.Body.Close()
	require.Equal(t, http.StatusSeeOther, resp2.StatusCode)

	updated := &models.Attachment{}
	require.NoError(t, tx.Find(updated, a.ID))
	require.False(t, updated.Comment.Valid, "whitespace-only comment clears the field")
}
