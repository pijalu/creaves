package models

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/validate"
	"github.com/gofrs/uuid"
)

// Attachment is a photo or video attached to an animal record (issue #34).
// The binary content lives on the local disk (storage_path is relative to
// the instance storage root); only metadata is persisted.
type Attachment struct {
	ID          uuid.UUID `json:"id" db:"id"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
	AnimalID    int       `json:"animal_id" db:"animal_id"`
	Filename    string    `json:"filename" db:"filename"`
	ContentType string    `json:"content_type" db:"content_type"`
	Size        int64     `json:"size" db:"size"`
	StoragePath string    `json:"-" db:"storage_path"`
	Kind        string    `json:"kind" db:"kind"`
	UploadedBy  nulls.UUID `json:"uploaded_by" db:"uploaded_by"`
}

// Attachments is a slice of Attachment.
type Attachments []Attachment

// Attachment kinds (denormalized from the content type for the gallery).
const (
	AttachmentKindImage = "image"
	AttachmentKindVideo = "video"
)

// allowedAttachmentTypes maps sniffed/declared MIME types to the gallery kind.
var allowedAttachmentTypes = map[string]string{
	"image/jpeg": AttachmentKindImage,
	"image/png":  AttachmentKindImage,
	"image/webp": AttachmentKindImage,
	"image/gif":  AttachmentKindImage,
	"video/mp4":  AttachmentKindVideo,
	"video/webm": AttachmentKindVideo,
}

// AttachmentKind returns the gallery kind for a content type, or "" when the
// type is not allowed.
func AttachmentKind(contentType string) string {
	return allowedAttachmentTypes[strings.ToLower(strings.TrimSpace(contentType))]
}

// Attachment max sizes: generous for photos, roomier for short videos.
var (
	AttachmentMaxImageSize int64 = 10 << 20 // 10 MB
	AttachmentMaxVideoSize int64 = 100 << 20 // 100 MB
)

// AttachmentMaxSize returns the size limit for a content type.
func AttachmentMaxSize(contentType string) int64 {
	if AttachmentKind(contentType) == AttachmentKindVideo {
		return AttachmentMaxVideoSize
	}
	return AttachmentMaxImageSize
}

// SanitizeFilename strips any directory component and characters that are
// unsafe in Content-Disposition headers or on disk. The original name is
// display-only (stored files are named by UUID), so aggressiveness is fine.
func SanitizeFilename(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.Map(func(r rune) rune {
		switch {
		case r < 0x20 || r == 0x7f:
			return -1
		case strings.ContainsRune(`<>:"/|?*`, r):
			return '_'
		case unicode.IsControl(r):
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	if len(name) > 200 {
		name = name[len(name)-200:]
	}
	if name == "" || name == "." || name == ".." {
		name = "attachment"
	}
	return name
}

// Validate gets run every time you call a "pop.Validate*" method.
func (a *Attachment) Validate(tx *pop.Connection) (*validate.Errors, error) {
	errs := validate.NewErrors()
	if a.AnimalID == 0 {
		errs.Add("animal_id", "animal must be set")
	}
	if strings.TrimSpace(a.Filename) == "" {
		errs.Add("filename", "filename must not be blank")
	}
	kind := AttachmentKind(a.ContentType)
	if kind == "" {
		errs.Add("content_type", fmt.Sprintf("content type %q is not allowed (images: jpeg/png/webp/gif, videos: mp4/webm)", a.ContentType))
		return errs, nil
	}
	max := AttachmentMaxSize(a.ContentType)
	if a.Size <= 0 {
		errs.Add("size", "size must be positive")
	} else if a.Size > max {
		errs.Add("size", fmt.Sprintf("file is too large (max %d bytes for %s)", max, a.ContentType))
	}
	if strings.TrimSpace(a.StoragePath) == "" {
		errs.Add("storage_path", "storage path must not be blank")
	}
	return errs, nil
}

// String is not required by pop and may be deleted.
func (a Attachment) String() string {
	return a.Filename
}

// Attachments is not required by pop and may be deleted.
func (a Attachments) String() string {
	return fmt.Sprintf("Attachments{%d}", len(a))
}
