package grainfs

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
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

func Test_DemoTest(t *testing.T) {
	// We want to exhaustively test the GrainFS functionality.
	// 1. Make a new GrainFS instance backed on disk.
	// 2. Add three directories.
	// 3. Add three files, one in each directory.
	// 4. Read the files back and verify their contents.
	// 5. Make a new GrainFS instance with the same password from the same disk.
	// 6. Read the files back and verify their contents again.
	// 7. Add a nested directory and a file in it.
	// 8. Read the nested file and verify its contents.
	// 9. List the directories and files, ensuring they match expected names.
	// 10. Update the contents of one file and verify the change.
	// 11. Remove a file and verify it no longer exists.
	// 12. Remove a directory and verify it no longer exists.
	underlying := osfs.New("test_grainfs")
	password := "test-password-123"
	fs, err := New(underlying, password)
	if err != nil {
		t.Fatalf("Failed to create GrainFS: %v", err)
	}
	// Step 1: Create directories
	dirs := []string{"dir1", "dir2", "dir3"}
	for _, dir := range dirs {
		err = fs.MkdirAll(dir, 0755)
		if err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}
	// Step 2: Create files in each directory
	files := map[string][]byte{
		"dir1/file1.txt": []byte("Content of file 1"),
		"dir2/file2.txt": []byte("Content of file 2"),
		"dir3/file3.txt": []byte("Content of file 3"),
	}
	for path, content := range files {
		file, err := fs.Create(path)
		if err != nil {
			t.Fatalf("Failed to create file %s: %v", path, err)
		}
		_, err = file.Write(content)
		if err != nil {
			t.Fatalf("Failed to write to file %s: %v", path, err)
		}
		err = file.Close()
		if err != nil {
			t.Fatalf("Failed to close file %s: %v", path, err)
		}
	}

	// Step 3: Read files back and verify contents
	for path, expectedContent := range files {
		file, err := fs.Open(path)
		if err != nil {
			t.Fatalf("Failed to open file %s: %v", path, err)
		}
		content, err := io.ReadAll(file)
		if err != nil {
			t.Fatalf("Failed to read file %s: %v", path, err)
		}
		err = file.Close()
		if err != nil {
			t.Fatalf("Failed to close file %s: %v", path, err)
		}
		if !bytes.Equal(content, expectedContent) {
			t.Fatalf("Content of file %s does not match expected content.\nExpected: %s\nGot: %s", path, expectedContent, content)
		}
	}

	// Step 4: Create a new GrainFS instance with the same password
	fs2, err := New(underlying, password)
	if err != nil {
		t.Fatalf("Failed to create second GrainFS instance: %v", err)
	}
	// Step 5: Read files back and verify contents again
	for path, expectedContent := range files {
		file, err := fs2.Open(path)
		if err != nil {
			t.Fatalf("Failed to open file %s in second instance: %v", path, err)
		}
		content, err := io.ReadAll(file)
		if err != nil {
			t.Fatalf("Failed to read file %s in second instance: %v", path, err)
		}
		err = file.Close()
		if err != nil {
			t.Fatalf("Failed to close file %s in second instance: %v", path, err)
		}
		if !bytes.Equal(content, expectedContent) {
			t.Fatalf("Content of file %s in second instance does not match expected content.\nExpected: %s\nGot: %s", path, expectedContent, content)
		}
	}

	// Step 6: Add a nested directory and a file in it
	nestedDir := "dir-nested/nested"
	err = fs.MkdirAll(nestedDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create nested directory %s: %v", nestedDir, err)
	}
	nestedFilePath := nestedDir + "/nested_file.txt"
	nestedFileContent := []byte("Content of nested file")
	nestedFile, err := fs.Create(nestedFilePath)
	if err != nil {
		t.Fatalf("Failed to create nested file %s: %v", nestedFilePath, err)
	}
	_, err = nestedFile.Write(nestedFileContent)
	if err != nil {
		t.Fatalf("Failed to write to nested file %s: %v", nestedFilePath, err)
	}
	err = nestedFile.Close()
	if err != nil {
		t.Fatalf("Failed to close nested file %s: %v", nestedFilePath, err)
	}

	// Step 7: Read the nested file and verify its contents
	nestedFile, err = fs.Open(nestedFilePath)
	if err != nil {
		t.Fatalf("Failed to open nested file %s: %v", nestedFilePath, err)
	}
	nestedContent, err := io.ReadAll(nestedFile)
	if err != nil {
		t.Fatalf("Failed to read nested file %s: %v", nestedFilePath, err)
	}

	if err = nestedFile.Close(); err != nil {
		t.Fatalf("Failed to close nested file %s: %v", nestedFilePath, err)
	}
	if !bytes.Equal(nestedContent, nestedFileContent) {
		t.Fatalf("Content of nested file %s does not match expected content.\nExpected: %s\nGot: %s", nestedFilePath, nestedFileContent, nestedContent)
	}

	// Step 8: List directories and files, ensuring they match expected names
	for _, dir := range dirs {
		entries, err := fs.ReadDir(dir)
		if err != nil {
			t.Fatalf("Failed to read directory %s: %v", dir, err)
		}
		if len(entries) != 1 {
			t.Fatalf("Expected 1 entry in directory %s, got %d, %s", dir, len(entries), entries)
		}
	}

	entries, err := fs.ReadDir(nestedDir)
	if err != nil {
		t.Fatalf("Failed to read nested directory %s: %v", nestedDir, err)
	}
	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry in nested directory %s, got %d", nestedDir, len(entries))
	}
	if entries[0].Name() != "nested_file.txt" {
		t.Fatalf("Expected nested file name 'nested_file.txt', got '%s'", entries[0].Name())
	}

	// Step 9: Update the contents of one file and verify the change
	updatedContent := []byte("Updated content of file 1")
	file1Path := "dir1/file1.txt"
	file1, err := fs.OpenFile(file1Path, os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		t.Fatalf("Failed to open file %s for update: %v", file1Path, err)
	}
	_, err = file1.Write(updatedContent)
	if err != nil {
		t.Fatalf("Failed to write updated content to file %s: %v", file1Path, err)
	}
	err = file1.Close()
	if err != nil {
		t.Fatalf("Failed to close file %s after update: %v", file1Path, err)
	}

	// Verify the updated content
	file1, err = fs.Open(file1Path)
	if err != nil {
		t.Fatalf("Failed to open updated file %s: %v", file1Path, err)
	}
	updatedData, err := io.ReadAll(file1)
	if err != nil {
		t.Fatalf("Failed to read updated file %s: %v", file1Path, err)
	}
	if err = file1.Close(); err != nil {
		t.Fatalf("Failed to close updated file %s: %v", file1Path, err)
	}
	if !bytes.Equal(updatedData, updatedContent) {
		t.Fatalf("Content of updated file %s does not match expected content.\nExpected: %s\nGot: %s", file1Path, updatedContent, updatedData)
	}

	// Step 10: Remove a file and verify it no longer exists
	if err = fs.Remove(nestedFilePath); err != nil {
		t.Fatalf("Failed to remove nested file %s: %v", nestedFilePath, err)
	}
	if _, err = fs.Stat(nestedFilePath); !os.IsNotExist(err) {
		t.Fatalf("Nested file %s should not exist after removal, but it does", nestedFilePath)
	}

	// Step 11: Remove a directory and verify it no longer exists
	if err = fs.Remove("dir2"); err == nil || !strings.Contains(err.Error(), "directory not empty") {
		t.Fatalf("Failed to get err when removing directory dir2: %v", err)
	}

	dir2Entries, err := fs.ReadDir("dir2")
	if err != nil {
		t.Fatalf("Failed to read directory dir2: %v", err)
	}
	for _, entry := range dir2Entries {
		if err := fs.Remove(filepath.Join("dir2", entry.Name())); err != nil {
			t.Fatalf("Failed to remove file %s in dir2: %v", entry.Name(), err)
		}
	}

	if err = fs.Remove("dir2"); err != nil {
		t.Fatalf("Failed to remove directory dir2: %v", err)
	}

	if _, err = fs.Stat("dir2"); !os.IsNotExist(err) {
		t.Fatalf("Directory dir2 should not exist after removal, but it does")
	}
	// Step 12: Verify remaining directories and files
	remainingDirs := []string{"dir1", "dir3"}
	for _, dir := range remainingDirs {
		if _, err := fs.Stat(dir); err != nil {
			t.Fatalf("Directory %s should exist, but it does not: %v", dir, err)
		}
	}
	// Verify files in remaining directories
	remainingFiles := map[string][]byte{
		"dir1/file1.txt": updatedContent,
		"dir3/file3.txt": files["dir3/file3.txt"],
	}
	for path, expectedContent := range remainingFiles {
		file, err := fs.Open(path)
		if err != nil {
			t.Fatalf("Failed to open remaining file %s: %v", path, err)
		}
		content, err := io.ReadAll(file)
		if err != nil {
			t.Fatalf("Failed to read remaining file %s: %v", path, err)
		}
		err = file.Close()
		if err != nil {
			t.Fatalf("Failed to close remaining file %s: %v", path, err)
		}
		if !bytes.Equal(content, expectedContent) {
			t.Fatalf("Content of remaining file %s does not match expected content.\nExpected: %s\nGot: %s", path, expectedContent, content)
		}
	}
	// Cleanup: Remove all remaining files and directories
	for path := range remainingFiles {
		err = fs.Remove(path)
		if err != nil {
			t.Fatalf("Failed to remove file %s: %v", path, err)
		}
	}

	_ = os.RemoveAll("test_grainfs") // Clean up the test directory
}
