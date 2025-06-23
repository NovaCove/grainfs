package grainfs

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// FilenameMap represents the mapping between original and obfuscated filenames
type FilenameMap map[string]string

// FilemapManager handles filename mapping operations
type FilemapManager struct {
	fs         *GrainFS
	cache      map[string]FilenameMap
	cacheMutex sync.RWMutex
}

// NewFilemapManager creates a new filename mapping manager
func NewFilemapManager(fs *GrainFS) *FilemapManager {
	return &FilemapManager{
		fs:    fs,
		cache: make(map[string]FilenameMap),
	}
}

// obfuscateFilename creates an obfuscated filename for the given directory and original filename
func (fs *GrainFS) obfuscateFilename(dir, filename string) (string, error) {
	if filename == "" {
		return "", fmt.Errorf("filename cannot be empty")
	}

	// Special handling for .grainfs directory and its contents
	if filename == GrainFSDir || strings.HasPrefix(filename, GrainFSDir+"/") {
		return filename, nil
	}

	// Get obfuscated name
	obfuscated, err := obfuscateFilename(fs.filenameKey, filename)
	if err != nil {
		return "", fmt.Errorf("failed to obfuscate filename: %w", err)
	}

	// Handle collisions by adding counter suffix
	finalObfuscated := obfuscated
	counter := 1
	for {
		// Check if this obfuscated name already exists in the filemap
		filemap, err := fs.loadFilemap(dir)
		if err != nil {
			return "", fmt.Errorf("failed to load filemap: %w", err)
		}

		// Check if the obfuscated name is already used for a different original filename
		if existingOriginal, exists := filemap[finalObfuscated]; exists {
			if existingOriginal == filename {
				// Same original filename, we can reuse this obfuscated name
				return finalObfuscated, nil
			}
			// Collision with different original filename, try with counter
			finalObfuscated = fmt.Sprintf("%s.%d", obfuscated, counter)
			counter++
			continue
		}

		// No collision, we can use this obfuscated name
		break
	}

	// Update the filemap with the new mapping
	if err := fs.updateFilemap(dir, filename, finalObfuscated); err != nil {
		return "", fmt.Errorf("failed to update filemap: %w", err)
	}

	return finalObfuscated, nil
}

// deobfuscateFilename resolves an obfuscated filename back to the original
func (fs *GrainFS) deobfuscateFilename(dir, obfuscated string) (string, error) {
	if obfuscated == "" {
		return "", fmt.Errorf("obfuscated filename cannot be empty")
	}

	// Special handling for .grainfs directory and its contents
	if obfuscated == GrainFSDir || strings.HasPrefix(obfuscated, GrainFSDir+"/") {
		return obfuscated, nil
	}

	filemap, err := fs.loadFilemap(dir)
	if err != nil {
		return "", fmt.Errorf("failed to load filemap: %w", err)
	}

	original, exists := filemap[obfuscated]
	if !exists {
		return "", fmt.Errorf("obfuscated filename not found in filemap: %s", obfuscated)
	}

	return original, nil
}

// updateFilemap updates the filename mapping for a directory
func (fs *GrainFS) updateFilemap(dir, original, obfuscated string) error {
	// Ensure .grainfs directory exists
	if err := fs.ensureGrainFSDir(dir); err != nil {
		return fmt.Errorf("failed to ensure .grainfs directory: %w", err)
	}

	// Load existing filemap
	filemap, err := fs.loadFilemap(dir)
	if err != nil {
		// If filemap doesn't exist, create a new one
		if os.IsNotExist(err) {
			filemap = make(FilenameMap)
		} else {
			return fmt.Errorf("failed to load existing filemap: %w", err)
		}
	}

	// Update the mapping
	filemap[obfuscated] = original

	// Save the updated filemap
	return fs.saveFilemap(dir, filemap)
}

// removeFromFilemap removes a filename mapping from the directory's filemap
func (fs *GrainFS) removeFromFilemap(dir, obfuscated string) error {
	filemap, err := fs.loadFilemap(dir)
	if err != nil {
		if os.IsNotExist(err) {
			// Filemap doesn't exist, nothing to remove
			return nil
		}
		return fmt.Errorf("failed to load filemap: %w", err)
	}

	// Remove the mapping
	delete(filemap, obfuscated)

	// Save the updated filemap
	return fs.saveFilemap(dir, filemap)
}

// loadFilemap loads the filename mapping for a directory
func (fs *GrainFS) loadFilemap(dir string) (FilenameMap, error) {
	// Check cache first
	fs.filemapManager.cacheMutex.RLock()
	if cached, exists := fs.filemapManager.cache[dir]; exists {
		fs.filemapManager.cacheMutex.RUnlock()
		return cached, nil
	}
	fs.filemapManager.cacheMutex.RUnlock()

	// Get the obfuscated directory path
	obfuscatedDir, err := fs.getObfuscatedPath(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to get obfuscated directory path: %w", err)
	}

	filemapPath := filepath.Join(obfuscatedDir, GrainFSDir, FilemapFile)

	file, err := fs.underlying.Open(filemapPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty filemap if it doesn't exist
			emptyMap := make(FilenameMap)
			// Cache the empty map
			fs.filemapManager.cacheMutex.Lock()
			fs.filemapManager.cache[dir] = emptyMap
			fs.filemapManager.cacheMutex.Unlock()
			return emptyMap, nil
		}
		return nil, fmt.Errorf("failed to open filemap: %w", err)
	}
	defer file.Close()

	// Read and decrypt the filemap
	encryptedData, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read filemap: %w", err)
	}

	// Decrypt the filemap data
	decryptedData, err := decryptData(fs.masterKey, encryptedData)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt filemap: %w", err)
	}

	var filemap FilenameMap
	if err := json.Unmarshal(decryptedData, &filemap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal filemap: %w", err)
	}

	// Cache the loaded filemap
	fs.filemapManager.cacheMutex.Lock()
	fs.filemapManager.cache[dir] = filemap
	fs.filemapManager.cacheMutex.Unlock()

	return filemap, nil
}

// saveFilemap saves the filename mapping for a directory
func (fs *GrainFS) saveFilemap(dir string, filemap FilenameMap) error {
	// Marshal the filemap to JSON
	jsonData, err := json.MarshalIndent(filemap, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal filemap: %w", err)
	}

	// Encrypt the filemap data
	encryptedData, err := encryptData(fs.masterKey, jsonData)
	if err != nil {
		return fmt.Errorf("failed to encrypt filemap: %w", err)
	}

	// Get the obfuscated directory path
	obfuscatedDir, err := fs.getObfuscatedPath(dir)
	if err != nil {
		return fmt.Errorf("failed to get obfuscated directory path: %w", err)
	}

	filemapPath := filepath.Join(obfuscatedDir, GrainFSDir, FilemapFile)

	file, err := fs.underlying.Create(filemapPath)
	if err != nil {
		return fmt.Errorf("failed to create filemap file: %w", err)
	}
	defer file.Close()

	if _, err := file.Write(encryptedData); err != nil {
		return fmt.Errorf("failed to write filemap: %w", err)
	}

	// Update cache
	fs.filemapManager.cacheMutex.Lock()
	fs.filemapManager.cache[dir] = filemap
	fs.filemapManager.cacheMutex.Unlock()

	return nil
}

// invalidateFilemapCache removes a directory's filemap from the cache
func (fs *GrainFS) invalidateFilemapCache(dir string) {
	fs.filemapManager.cacheMutex.Lock()
	delete(fs.filemapManager.cache, dir)
	fs.filemapManager.cacheMutex.Unlock()
}

// getObfuscatedPath converts a user path to the obfuscated path on disk
func (fs *GrainFS) getObfuscatedPath(userPath string) (string, error) {
	if userPath == "" || userPath == "." {
		return ".", nil
	}

	// Clean the path
	userPath = filepath.Clean(userPath)

	// Split path into components
	parts := strings.Split(userPath, string(filepath.Separator))
	obfuscatedParts := make([]string, 0, len(parts))

	currentDir := "."
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}

		// Obfuscate this part
		obfuscatedPart, err := fs.obfuscateFilename(currentDir, part)
		if err != nil {
			return "", fmt.Errorf("failed to obfuscate path component %q: %w", part, err)
		}

		obfuscatedParts = append(obfuscatedParts, obfuscatedPart)
		currentDir = filepath.Join(currentDir, obfuscatedPart)
	}

	if len(obfuscatedParts) == 0 {
		return ".", nil
	}

	return filepath.Join(obfuscatedParts...), nil
}

// getUserPath converts an obfuscated path back to the user path
func (fs *GrainFS) getUserPath(obfuscatedPath string) (string, error) {
	if obfuscatedPath == "" || obfuscatedPath == "." {
		return ".", nil
	}

	// Clean the path
	obfuscatedPath = filepath.Clean(obfuscatedPath)

	// Split path into components
	parts := strings.Split(obfuscatedPath, string(filepath.Separator))
	userParts := make([]string, 0, len(parts))

	currentDir := "."
	for i, part := range parts {
		if part == "" || part == "." {
			continue
		}

		// Deobfuscate this part
		userPart, err := fs.deobfuscateFilename(currentDir, part)
		if err != nil {
			return "", fmt.Errorf("failed to deobfuscate path component %q: %w", part, err)
		}

		userParts = append(userParts, userPart)

		// Update current directory for next iteration
		if i < len(parts)-1 {
			currentDir = filepath.Join(currentDir, part)
		}
	}

	if len(userParts) == 0 {
		return ".", nil
	}

	return filepath.Join(userParts...), nil
}
