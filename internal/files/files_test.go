package files

import (
	"errors"
	"regexp"
	"testing"
	"time"
)

func TestCleanName(t *testing.T) {
	cases := map[string]string{
		"../../etc/passwd":         "passwd",
		`C:\Users\a\contract.docx`: "contract.docx",
		"  notes.txt ":             "notes.txt",
	}
	for in, want := range cases {
		if got, err := CleanName(in); err != nil || got != want {
			t.Errorf("CleanName(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "..", "/", "a\x00b"} {
		if _, err := CleanName(bad); err == nil {
			t.Errorf("CleanName(%q) should fail", bad)
		}
	}
}

func TestPolicyCheck(t *testing.T) {
	p := Policy{AllowedExt: map[string]bool{"pdf": true, "png": true, "txt": true}, MaxFileBytes: 1024}
	pdf := []byte("%PDF-1.7\n")
	png := []byte("\x89PNG\r\n\x1a\n0000")

	if ct, err := p.Check("report.pdf", 9, pdf); err != nil || ct != "application/pdf" {
		t.Errorf("valid pdf: %q %v", ct, err)
	}
	if _, err := p.Check("photo.png", 12, png); err != nil {
		t.Errorf("valid png: %v", err)
	}
	checks := []struct {
		name string
		size int64
		head []byte
		want error
	}{
		{"tool.exe", 10, []byte("MZ"), ErrExtNotAllowed},
		{"noext", 10, pdf, ErrExtNotAllowed},
		{"big.pdf", 2048, pdf, ErrTooLarge},
		{"empty.txt", 0, nil, ErrEmpty},
		{"fake.pdf", 10, []byte("hello world"), ErrContentInvalid},
		{"fake.png", 10, pdf, ErrContentInvalid},
		{"elf.txt", 10, []byte("\x7fELF...."), ErrContentInvalid},
	}
	for _, c := range checks {
		if _, err := p.Check(c.name, c.size, c.head); !errors.Is(err, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, err, c.want)
		}
	}
}

func TestObjectKey(t *testing.T) {
	id := NewID()
	if !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`).MatchString(id) {
		t.Fatalf("bad uuid %q", id)
	}
	key := ObjectKey(id, "Annual Report.PDF", time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC))
	if key != "files/2026/11/"+id+".pdf" {
		t.Errorf("key = %q", key)
	}
}

func TestKindOf(t *testing.T) {
	for name, want := range map[string]string{"a.pdf": KindDocument, "b.JPG": KindImage, "c.xlsx": KindSpreadsheet, "d.zip": KindArchive, "e.bin": KindOther} {
		if got := KindOf(name); got != want {
			t.Errorf("KindOf(%q) = %q, want %q", name, got, want)
		}
	}
}
