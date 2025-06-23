package grainfs

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-billy/v5/osfs"
)

func TestGrainFSBasicOperations(t *testing.T) {
	// Create a memory filesystem for testing
	underlying := memfs.New()
	password := "test-password-123"

	// Create GrainFS
	fs, err := New(underlying, password)
	if err != nil {
		t.Fatalf("Failed to create GrainFS: %v", err)
	}

	// Test file creation and writing
	testData := []byte("Hello, GrainFS! This is encrypted data.")
	filename := "test.txt"

	file, err := fs.Create(filename)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	n, err := file.Write(testData)
	if err != nil {
		t.Fatalf("Failed to write to file: %v", err)
	}
	if n != len(testData) {
		t.Fatalf("Expected to write %d bytes, wrote %d", len(testData), n)
	}

	err = file.Close()
	if err != nil {
		t.Fatalf("Failed to close file: %v", err)
	}

	// Test file reading
	file, err = fs.Open(filename)
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}

	readData, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	err = file.Close()
	if err != nil {
		t.Fatalf("Failed to close file: %v", err)
	}

	if !bytes.Equal(testData, readData) {
		t.Fatalf("Read data doesn't match written data.\nExpected: %s\nGot: %s", testData, readData)
	}

	// Test file stat
	info, err := fs.Stat(filename)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	if info.Name() != filename {
		t.Fatalf("Expected filename %s, got %s", filename, info.Name())
	}

	// Test file removal
	err = fs.Remove(filename)
	if err != nil {
		t.Fatalf("Failed to remove file: %v", err)
	}

	// Verify file is removed
	_, err = fs.Stat(filename)
	if !os.IsNotExist(err) {
		t.Fatalf("File should not exist after removal")
	}
}

func TestGrainFSDirectoryOperations(t *testing.T) {
	underlying := memfs.New()
	password := "test-password-123"

	fs, err := New(underlying, password)
	if err != nil {
		t.Fatalf("Failed to create GrainFS: %v", err)
	}

	// Test directory creation
	dirPath := "testdir"
	err = fs.MkdirAll(dirPath, 0755)
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	// Create files in directory
	file1Path := filepath.Join(dirPath, "file1.txt")
	file2Path := filepath.Join(dirPath, "file2.txt")

	// Create first file
	file1, err := fs.Create(file1Path)
	if err != nil {
		t.Fatalf("Failed to create file1: %v", err)
	}
	file1.Write([]byte("content1"))
	file1.Close()

	// Create second file
	file2, err := fs.Create(file2Path)
	if err != nil {
		t.Fatalf("Failed to create file2: %v", err)
	}
	file2.Write([]byte("content2"))
	file2.Close()

	// Test directory reading
	infos, err := fs.ReadDir(dirPath)
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}

	if len(infos) != 2 {
		t.Fatalf("Expected 2 files in directory, got %d", len(infos))
	}

	// Check filenames are properly deobfuscated
	names := make(map[string]bool)
	for _, info := range infos {
		names[info.Name()] = true
	}

	if !names["file1.txt"] || !names["file2.txt"] {
		t.Fatalf("Expected files file1.txt and file2.txt, got: %v", names)
	}
}

func TestGrainFSEncryption(t *testing.T) {
	underlying := memfs.New()
	password := "test-password-123"

	fs, err := New(underlying, password)
	if err != nil {
		t.Fatalf("Failed to create GrainFS: %v", err)
	}

	// Write some data
	filename := "secret.txt"
	secretData := []byte("This is very secret information that should be encrypted!")

	file, err := fs.Create(filename)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	file.Write(secretData)
	file.Close()

	// Check that the underlying filesystem has encrypted data
	// First, we need to find the obfuscated filename
	infos, err := underlying.ReadDir(".")
	if err != nil {
		t.Fatalf("Failed to read underlying directory: %v", err)
	}

	var encryptedFilename string
	for _, info := range infos {
		if info.Name() != GrainFSDir {
			encryptedFilename = info.Name()
			break
		}
	}

	if encryptedFilename == "" {
		t.Fatalf("No encrypted file found in underlying filesystem")
	}

	// Read the raw encrypted data
	rawFile, err := underlying.Open(encryptedFilename)
	if err != nil {
		t.Fatalf("Failed to open raw encrypted file: %v", err)
	}

	rawData, err := io.ReadAll(rawFile)
	rawFile.Close()
	if err != nil {
		t.Fatalf("Failed to read raw encrypted data: %v", err)
	}

	// Verify the raw data is different from the original (encrypted)
	if bytes.Equal(rawData, secretData) {
		t.Fatalf("Raw data should be encrypted, but it matches the original data")
	}

	// Verify we can still read the decrypted data through GrainFS
	file, err = fs.Open(filename)
	if err != nil {
		t.Fatalf("Failed to open file through GrainFS: %v", err)
	}

	decryptedData, err := io.ReadAll(file)
	file.Close()
	if err != nil {
		t.Fatalf("Failed to read decrypted data: %v", err)
	}

	if !bytes.Equal(decryptedData, secretData) {
		t.Fatalf("Decrypted data doesn't match original data")
	}
}

func TestGrainFSPasswordProtection(t *testing.T) {
	underlying := memfs.New()
	password1 := "correct-password"
	password2 := "wrong-password"

	// Create filesystem with first password
	fs1, err := New(underlying, password1)
	if err != nil {
		t.Fatalf("Failed to create GrainFS with password1: %v", err)
	}

	// Write some data
	filename := "protected.txt"
	testData := []byte("This data is protected by password")

	file, err := fs1.Create(filename)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	file.Write(testData)
	file.Close()

	// Try to access with wrong password
	fs2, err := New(underlying, password2)
	if err != nil {
		t.Fatalf("Failed to create GrainFS with password2: %v", err)
	}

	// This should fail to decrypt properly
	file, err = fs2.Open(filename)
	if err == nil {
		// If we can open the file, reading should fail or return garbage
		data, readErr := io.ReadAll(file)
		file.Close()

		if readErr == nil && bytes.Equal(data, testData) {
			t.Fatalf("Should not be able to read correct data with wrong password")
		}
	}

	// Verify correct password still works
	file, err = fs1.Open(filename)
	if err != nil {
		t.Fatalf("Failed to open file with correct password: %v", err)
	}

	correctData, err := io.ReadAll(file)
	file.Close()
	if err != nil {
		t.Fatalf("Failed to read with correct password: %v", err)
	}

	if !bytes.Equal(correctData, testData) {
		t.Fatalf("Data read with correct password doesn't match original")
	}
}

func TestGrainFSRename(t *testing.T) {
	underlying := memfs.New()
	password := "test-password-123"

	fs, err := New(underlying, password)
	if err != nil {
		t.Fatalf("Failed to create GrainFS: %v", err)
	}

	// Create a file
	oldName := "old.txt"
	newName := "new.txt"
	testData := []byte("test data for rename")

	file, err := fs.Create(oldName)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	file.Write(testData)
	file.Close()

	// Rename the file
	err = fs.Rename(oldName, newName)
	if err != nil {
		t.Fatalf("Failed to rename file: %v", err)
	}

	// Verify old name doesn't exist
	_, err = fs.Stat(oldName)
	if !os.IsNotExist(err) {
		t.Fatalf("Old filename should not exist after rename")
	}

	// Verify new name exists and has correct content
	file, err = fs.Open(newName)
	if err != nil {
		t.Fatalf("Failed to open renamed file: %v", err)
	}

	data, err := io.ReadAll(file)
	file.Close()
	if err != nil {
		t.Fatalf("Failed to read renamed file: %v", err)
	}

	if !bytes.Equal(data, testData) {
		t.Fatalf("Renamed file content doesn't match original")
	}
}

func TestGrainFSReadAt(t *testing.T) {
	underlying := memfs.New()
	password := "test-password-123"

	fs, err := New(underlying, password)
	if err != nil {
		t.Fatalf("Failed to create GrainFS: %v", err)
	}

	// Create a file with known content
	filename := "readat.txt"
	testData := []byte("0123456789abcdefghijklmnopqrstuvwxyz")

	file, err := fs.Create(filename)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	file.Write(testData)
	file.Close()

	// Test ReadAt
	file, err = fs.Open(filename)
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	// Read 5 bytes starting at offset 10
	buf := make([]byte, 5)
	n, err := file.ReadAt(buf, 10)
	if err != nil {
		t.Fatalf("ReadAt failed: %v", err)
	}
	if n != 5 {
		t.Fatalf("Expected to read 5 bytes, got %d", n)
	}

	expected := testData[10:15]
	if !bytes.Equal(buf, expected) {
		t.Fatalf("ReadAt returned wrong data. Expected %s, got %s", expected, buf)
	}
}

// Benchmark test to ensure reasonable performance
func BenchmarkGrainFSWrite(b *testing.B) {
	underlying := memfs.New()
	password := "benchmark-password"

	fs, err := New(underlying, password)
	if err != nil {
		b.Fatalf("Failed to create GrainFS: %v", err)
	}

	data := make([]byte, 1024) // 1KB of data
	for i := range data {
		data[i] = byte(i % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		filename := "bench.txt"
		file, err := fs.Create(filename)
		if err != nil {
			b.Fatalf("Failed to create file: %v", err)
		}

		_, err = file.Write(data)
		if err != nil {
			b.Fatalf("Failed to write data: %v", err)
		}

		err = file.Close()
		if err != nil {
			b.Fatalf("Failed to close file: %v", err)
		}

		// Clean up
		fs.Remove(filename)
	}
}

func BenchmarkGrainFSRead(b *testing.B) {
	underlying := memfs.New()
	password := "benchmark-password"

	fs, err := New(underlying, password)
	if err != nil {
		b.Fatalf("Failed to create GrainFS: %v", err)
	}

	// Setup: create a file to read
	filename := "bench-read.txt"
	data := make([]byte, 1024) // 1KB of data
	for i := range data {
		data[i] = byte(i % 256)
	}

	file, err := fs.Create(filename)
	if err != nil {
		b.Fatalf("Failed to create file: %v", err)
	}
	file.Write(data)
	file.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		file, err := fs.Open(filename)
		if err != nil {
			b.Fatalf("Failed to open file: %v", err)
		}

		_, err = io.ReadAll(file)
		if err != nil {
			b.Fatalf("Failed to read data: %v", err)
		}

		err = file.Close()
		if err != nil {
			b.Fatalf("Failed to close file: %v", err)
		}
	}
}

// Test with real filesystem (optional, can be skipped in CI)
func TestGrainFSWithOSFS(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping OSFS test in short mode")
	}

	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "grainfs-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	underlying := osfs.New(tempDir)
	password := "osfs-test-password"

	fs, err := New(underlying, password)
	if err != nil {
		t.Fatalf("Failed to create GrainFS: %v", err)
	}

	// Basic test with real filesystem
	filename := "osfs-test.txt"
	testData := []byte("Testing with real filesystem")

	file, err := fs.Create(filename)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	file.Write(testData)
	file.Close()

	file, err = fs.Open(filename)
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}

	readData, err := io.ReadAll(file)
	file.Close()
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if !bytes.Equal(testData, readData) {
		t.Fatalf("Data mismatch with OSFS")
	}

	// Verify files are actually encrypted on disk
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("Failed to read temp dir: %v", err)
	}

	// Should have .grainfs directory and obfuscated file
	foundGrainFS := false
	foundObfuscatedFile := false

	for _, entry := range entries {
		if entry.Name() == GrainFSDir {
			foundGrainFS = true
		} else if entry.Name() != filename { // Should be obfuscated, not original name
			foundObfuscatedFile = true
		}
	}

	if !foundGrainFS {
		t.Fatalf("Should have .grainfs directory")
	}
	if !foundObfuscatedFile {
		t.Fatalf("Should have obfuscated file, not original filename")
	}
}
