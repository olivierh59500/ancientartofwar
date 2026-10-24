package disk

import (
	"crypto/sha256"
	"fmt"
	"os"
	"testing"
)

func originalDisk(t testing.TB) []byte {
	t.Helper()
	b, err := os.ReadFile("../../assets/original/disk1.img")
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func TestExtractOriginalDisk(t *testing.T) {
	b := originalDisk(t)
	files, err := Read(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 57 {
		t.Fatalf("files=%d", len(files))
	}
	for _, f := range files {
		if f.Name == "WAR.EXE" {
			if got := fmt.Sprintf("%x", sha256.Sum256(f.Data)); got != "e55a202df53ec1d1beba20f0aef75f86a286987cc53fbe4b354a10c50b6c939e" {
				t.Fatalf("executable extraction hash %s", got)
			}
			return
		}
	}
	t.Fatal("missing WAR.EXE")
}
func TestMalformedDisks(t *testing.T) {
	for _, n := range []int{0, 511, 512, 1024, 6000} {
		if _, err := Read(make([]byte, n)); err == nil {
			t.Errorf("accepted malformed disk of size %d", n)
		}
	}
	b := originalDisk(t)
	b[512+3] = 2
	b[512+4] &= 0xf0 // cluster 2 points to itself
	if _, err := Read(b); err == nil {
		t.Fatal("accepted cyclic FAT chain")
	}
	b = originalDisk(t)
	b[512+3] = 0xff
	b[512+4] |= 15
	if _, err := Read(b); err == nil {
		t.Fatal("accepted premature end of cluster chain")
	}
}
func FuzzRead(f *testing.F) {
	f.Add(originalDisk(f))
	f.Add([]byte("bad disk"))
	f.Fuzz(func(t *testing.T, b []byte) { Read(b) })
}
