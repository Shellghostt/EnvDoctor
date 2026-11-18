//go:build windows

package bundle

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestM4WindowsAtomicReplaceLockedDestination(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "capture.zip")
	previous := []byte("previous regular destination")
	if err := os.WriteFile(path, previous, 0600); err != nil {
		t.Fatal(err)
	}
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := windows.CreateFile(p, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = windows.CloseHandle(h) }()
	if err := Publish(context.Background(), path, publicationArchive(t)); !errors.Is(err, ErrWrite) {
		t.Fatalf("locked replacement: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, previous) {
		t.Fatal("locked destination changed")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatal("locked replacement retained temporary file")
	}
}

func TestM4WindowsPublicationHasPrivateProtectedACL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capture.zip")
	if err := Publish(context.Background(), path, publicationArchive(t)); err != nil {
		t.Fatal(err)
	}
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	control, _, err := sd.Control()
	if err != nil {
		t.Fatal(err)
	}
	if control&windows.SE_DACL_PROTECTED == 0 {
		t.Fatal("staged publication inherited a non-private ACL")
	}
	dacl, _, err := sd.DACL()
	if err != nil || dacl == nil {
		t.Fatal("private DACL missing")
	}
}
