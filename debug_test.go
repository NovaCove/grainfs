package grainfs

import (
	"fmt"
	"testing"

	"github.com/go-git/go-billy/v5/memfs"
)

func TestDebugFilemapping(t *testing.T) {
	underlying := memfs.New()
	password := "test-password"

	fs, err := New(underlying, password)
	if err != nil {
		t.Fatalf("Failed to create GrainFS: %v", err)
	}

	filename := "test.txt"

	// Debug: Check what happens during file creation
	fmt.Printf("Creating file: %s\n", filename)

	// Get obfuscated path for creation
	obfuscatedPath, err := fs.getObfuscatedPath(filename)
	if err != nil {
		t.Fatalf("Failed to get obfuscated path: %v", err)
	}
	fmt.Printf("Obfuscated path: %s\n", obfuscatedPath)

	// Create the file
	file, err := fs.Create(filename)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	_, err = file.Write([]byte("test data"))
	if err != nil {
		t.Fatalf("Failed to write: %v", err)
	}

	err = file.Close()
	if err != nil {
		t.Fatalf("Failed to close: %v", err)
	}

	// Debug: Check filemap after creation
	filemap, err := fs.loadFilemap(".")
	if err != nil {
		t.Fatalf("Failed to load filemap: %v", err)
	}
	fmt.Printf("Filemap after creation: %+v\n", filemap)

	// Debug: Check what files exist in underlying filesystem
	infos, err := underlying.ReadDir(".")
	if err != nil {
		t.Fatalf("Failed to read underlying dir: %v", err)
	}
	fmt.Printf("Files in underlying filesystem:\n")
	for _, info := range infos {
		fmt.Printf("  %s\n", info.Name())
	}

	// Try to get obfuscated path for opening
	obfuscatedPath2, err := fs.getObfuscatedPath(filename)
	if err != nil {
		t.Fatalf("Failed to get obfuscated path for opening: %v", err)
	}
	fmt.Printf("Obfuscated path for opening: %s\n", obfuscatedPath2)

	// Try to open the file
	file2, err := fs.Open(filename)
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}
	file2.Close()

	fmt.Printf("Success!\n")
}
