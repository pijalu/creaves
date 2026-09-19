package models

import (
	"strings"
	"testing"
)

func TestAttachmentKind(t *testing.T) {
	tests := []struct {
		ct   string
		want string
	}{
		{"image/jpeg", AttachmentKindImage},
		{"image/png", AttachmentKindImage},
		{"image/webp", AttachmentKindImage},
		{"image/gif", AttachmentKindImage},
		{"video/mp4", AttachmentKindVideo},
		{"video/webm", AttachmentKindVideo},
		{"IMAGE/PNG", AttachmentKindImage}, // case insensitive
		{"application/pdf", ""},
		{"image/svg+xml", ""}, // svg is scriptable: never allowed
		{"text/html", ""},
		{"", ""},
	}
	for _, tt := range tests {
		if got := AttachmentKind(tt.ct); got != tt.want {
			t.Errorf("AttachmentKind(%q) = %q; want %q", tt.ct, got, tt.want)
		}
	}
}

func TestAttachmentValidate(t *testing.T) {
	valid := &Attachment{
		AnimalID:    42,
		Filename:    "photo.jpg",
		ContentType: "image/png",
		Size:        1024,
		StoragePath: "animal-42/x.png",
	}
	if verrs, err := valid.Validate(nil); err != nil || verrs.HasAny() {
		t.Fatalf("valid attachment rejected: %v %v", verrs, err)
	}

	tests := []struct {
		name string
		mut  func(*Attachment)
	}{
		{"no animal", func(a *Attachment) { a.AnimalID = 0 }},
		{"blank filename", func(a *Attachment) { a.Filename = " " }},
		{"bad type", func(a *Attachment) { a.ContentType = "application/pdf" }},
		{"zero size", func(a *Attachment) { a.Size = 0 }},
		{"oversize image", func(a *Attachment) { a.Size = AttachmentMaxImageSize + 1 }},
		{"blank storage path", func(a *Attachment) { a.StoragePath = "" }},
	}
	for _, tt := range tests {
		a := *valid
		tt.mut(&a)
		verrs, err := a.Validate(nil)
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", tt.name, err)
		}
		if !verrs.HasAny() {
			t.Errorf("%s: expected validation errors", tt.name)
		}
	}

	// video allowance: up to the video limit
	v := *valid
	v.ContentType = "video/mp4"
	v.Size = AttachmentMaxVideoSize
	if verrs, _ := v.Validate(nil); verrs.HasAny() {
		t.Errorf("video at limit rejected: %v", verrs)
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"photo.jpg", "photo.jpg"},
		{"/etc/passwd", "passwd"},
		{"..\\..\\windows\\evil.png", "evil.png"}, // backslash treated as separator
		{"../../secret.txt", "secret.txt"},
		{"my <photo>.jpg", "my _photo_.jpg"},
		{"ctrl\x00char.png", "ctrlchar.png"},
		{"  spaced.png  ", "spaced.png"},
		{"", "attachment"},
		{".", "attachment"},
		{"..", "attachment"},
	}
	for _, tt := range tests {
		if got := SanitizeFilename(tt.in); got != tt.want {
			t.Errorf("SanitizeFilename(%q) = %q; want %q", tt.in, got, tt.want)
		}
	}
	// never grows beyond the cap
	long := SanitizeFilename(strings.Repeat("a", 500) + ".jpg")
	if len(long) > 200 {
		t.Errorf("SanitizeFilename long name = %d chars; want <= 200", len(long))
	}
}
