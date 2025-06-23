package grainfs

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/go-git/go-billy/v5/memfs"
)

func TestDebugDirectoryOperations(t *testing.T) {
	underlying := memfs.New()
	password := "test-password-123"

	fs, err := New(underlying, password)
	if err != nil {
		t.Fatalf("Failed to create GrainFS: %v", err)
	}

	// Test directory creation
	dirPath := "testdir"
	fmt.Printf("Creating directory: %s\n", dirPath)
	err = fs.MkdirAll(dirPath, 0755)
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	// Create files in directory
	file1Path := filepath.Join(dirPath, "file1.txt")
	file2Path := filepath.Join(dirPath, "file2.txt")

	fmt.Printf("Creating file: %s\n", file1Path)
	file1, err := fs.Create(file1Path)
	if err != nil {
		t.Fatalf("Failed to create file1: %v", err)
	}
	file1.Write([]byte("content1"))
	file1.Close()

	fmt.Printf("Creating file: %s\n", file2Path)
	file2, err := fs.Create(file2Path)
	if err != nil {
		t.Fatalf("Failed to create file2: %v", err)
	}
	file2.Write([]byte("content2"))
	file2.Close()

	// Debug: Check what's in the underlying filesystem
	fmt.Printf("Files in underlying root:\n")
	infos, err := underlying.ReadDir(".")
	if err != nil {
		t.Fatalf("Failed to read underlying root: %v", err)
	}
	for _, info := range infos {
		fmt.Printf("  %s (dir: %v)\n", info.Name(), info.IsDir())
	}

	// Get obfuscated directory path
	obfuscatedDirPath, err := fs.getObfuscatedPath(dirPath)
	if err != nil {
		t.Fatalf("Failed to get obfuscated dir path: %v", err)
	}
	fmt.Printf("Obfuscated directory path: %s\n", obfuscatedDirPath)

	// Check what's in the obfuscated directory
	fmt.Printf("Files in obfuscated directory:\n")
	infos, err = underlying.ReadDir(obfuscatedDirPath)
	if err != nil {
		t.Fatalf("Failed to read obfuscated directory: %v", err)
	}
	for _, info := range infos {
		fmt.Printf("  %s (dir: %v)\n", info.Name(), info.IsDir())
	}

	// Check filemap for the directory
	filemap, err := fs.loadFilemap(dirPath)
	if err != nil {
		t.Fatalf("Failed to load filemap for directory: %v", err)
	}
	fmt.Printf("Filemap for directory %s: %+v\n", dirPath, filemap)

	// Test directory reading through GrainFS
	fmt.Printf("Reading directory through GrainFS:\n")
	infos, err = fs.ReadDir(dirPath)
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}
	fmt.Printf("Found %d files:\n", len(infos))
	for _, info := range infos {
		fmt.Printf("  %s (dir: %v)\n", info.Name(), info.IsDir())
	}
}
