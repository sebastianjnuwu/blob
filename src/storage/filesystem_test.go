package storage

import (
	"bytes"
	"context"
	"testing"
)

func TestFilesystemDriverBasics(t *testing.T) {
	root := t.TempDir()
	d := NewFilesystemDriver(root)
	if d.Name() != "filesystem" {
		t.Fatalf("name=%q", d.Name())
	}

	if got := validateRelativePathComponent("bucket", ""); got == nil {
		t.Fatal("empty bucket accepted")
	}
	if got := validateRelativePathComponent("bucket", "../x"); got == nil {
		t.Fatal("traversal accepted")
	}
	if _, err := d.resolve("../b", "key"); err == nil {
		t.Fatal("resolve accepted traversal")
	}

	ctx := context.Background()
	if err := d.EnsureBucket(ctx, "demo"); err != nil {
		t.Fatal(err)
	}
	if err := d.Put(ctx, "demo", "a/b.txt", bytes.NewBufferString("hello"), 5); err != nil {
		t.Fatal(err)
	}
	exists, err := d.Exists(ctx, "demo", "a/b.txt")
	if err != nil || !exists {
		t.Fatalf("exists=%v err=%v", exists, err)
	}
	info, err := d.Stat(ctx, "demo", "a/b.txt")
	if err != nil || info.Size != 5 {
		t.Fatalf("stat=%v err=%v", info, err)
	}
	f, err := d.Open(ctx, "demo", "a/b.txt")
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 5)
	if _, err := f.Read(buf); err != nil || string(buf) != "hello" {
		t.Fatalf("read=%q err=%v", buf, err)
	}
	f.Close()

	if err := d.Move(ctx, "demo", "a/b.txt", "demo", "c/d.txt"); err != nil {
		t.Fatal(err)
	}
	if exists, _ := d.Exists(ctx, "demo", "a/b.txt"); exists {
		t.Fatal("source still exists")
	}
	if exists, _ := d.Exists(ctx, "demo", "c/d.txt"); !exists {
		t.Fatal("destination missing")
	}

	if err := d.Delete(ctx, "demo", "c/d.txt"); err != nil {
		t.Fatal(err)
	}
	if exists, _ := d.Exists(ctx, "demo", "c/d.txt"); exists {
		t.Fatal("delete failed")
	}

	if err := d.RemoveTree(ctx, "demo", "unused"); err != nil {
		// nonexistent prefix is not an error
	}
}
