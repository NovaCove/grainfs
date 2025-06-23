package grainfs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/go-git/go-billy/v5"
)

// GrainFS implements an encrypted filesystem that wraps any billy.Filesystem
type GrainFS struct {
	underlying     billy.Filesystem
	masterKey      []byte
	filenameKey    []byte
	rootPath       string
	filemapManager *FilemapManager
	mutex          sync.RWMutex
}

// New creates a new GrainFS instance with the given underlying filesystem and password
func New(underlying billy.Filesystem, password string) (*GrainFS, error) {
	if underlying == nil {
		return nil, fmt.Errorf("underlying filesystem cannot be nil")
	}
	if password == "" {
		return nil, fmt.Errorf("password cannot be empty")
	}

	fs := &GrainFS{
		underlying: underlying,
		rootPath:   ".",
	}

	// Load or create configuration
	config, err := fs.loadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Derive keys from password and salt
	fs.masterKey, fs.filenameKey = deriveKeys(password, config.Salt, config.Iterations)

	// Initialize filemap manager
	fs.filemapManager = NewFilemapManager(fs)

	return fs, nil
}

// Ensure GrainFS implements all required billy interfaces
var (
	_ billy.Filesystem = (*GrainFS)(nil)
	_ billy.Basic      = (*GrainFS)(nil)
	_ billy.Dir        = (*GrainFS)(nil)
	_ billy.Symlink    = (*GrainFS)(nil)
	_ billy.Chroot     = (*GrainFS)(nil)
	_ billy.TempFile   = (*GrainFS)(nil)
)

// Basic interface implementation

// Create creates a new file with the given filename
func (fs *GrainFS) Create(filename string) (billy.File, error) {
	return fs.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
}

// Open opens the named file for reading
func (fs *GrainFS) Open(filename string) (billy.File, error) {
	return fs.OpenFile(filename, os.O_RDONLY, 0)
}

// OpenFile opens a file with the specified flag and perm
func (fs *GrainFS) OpenFile(filename string, flag int, perm os.FileMode) (billy.File, error) {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()

	return fs.openFileInternal(filename, flag, perm)
}

// openFileInternal is the internal implementation that doesn't acquire locks
func (fs *GrainFS) openFileInternal(filename string, flag int, perm os.FileMode) (billy.File, error) {
	if filename == "" {
		return nil, fmt.Errorf("filename cannot be empty")
	}

	// For file creation, we need to ensure the filemap is updated
	isCreating := (flag & os.O_CREATE) != 0

	var obfuscatedPath string
	var err error

	if isCreating {
		// When creating, use obfuscateFilename to update the filemap
		dir := filepath.Dir(filename)
		if dir == filename {
			dir = "."
		}
		basename := filepath.Base(filename)

		// Ensure directory exists and get its obfuscated path
		if dir != "." {
			if err := fs.mkdirAllInternal(dir, 0755); err != nil {
				return nil, fmt.Errorf("failed to create directory: %w", err)
			}
		}

		obfuscatedDir, err := fs.getObfuscatedPath(dir)
		if err != nil {
			return nil, fmt.Errorf("failed to get obfuscated directory path: %w", err)
		}

		obfuscatedBasename, err := fs.obfuscateFilename(dir, basename)
		if err != nil {
			return nil, fmt.Errorf("failed to obfuscate filename: %w", err)
		}

		obfuscatedPath = filepath.Join(obfuscatedDir, obfuscatedBasename)
	} else {
		// For opening existing files, just get the path without updating filemap
		obfuscatedPath, err = fs.getObfuscatedPath(filename)
		if err != nil {
			return nil, fmt.Errorf("failed to get obfuscated path: %w", err)
		}
	}

	// Open the underlying file
	underlyingFile, err := fs.underlying.OpenFile(obfuscatedPath, flag, perm)
	if err != nil {
		return nil, err
	}

	// Create encrypted file wrapper
	encFile := &EncryptedFile{
		underlying:  underlyingFile,
		fs:          fs,
		filename:    filename,
		obfuscated:  obfuscatedPath,
		flag:        flag,
		isWriteMode: (flag&os.O_WRONLY) != 0 || (flag&os.O_RDWR) != 0,
	}

	return encFile, nil
}

// Stat returns file information
func (fs *GrainFS) Stat(filename string) (os.FileInfo, error) {
	fs.mutex.RLock()
	defer fs.mutex.RUnlock()

	obfuscatedPath, err := fs.getObfuscatedPath(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to get obfuscated path: %w", err)
	}

	info, err := fs.underlying.Stat(obfuscatedPath)
	if err != nil {
		return nil, err
	}

	// Return a wrapped FileInfo that shows the original filename
	return &FileInfoWrapper{
		FileInfo:     info,
		originalName: filepath.Base(filename),
	}, nil
}

// Rename renames a file
func (fs *GrainFS) Rename(oldpath, newpath string) error {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()

	if oldpath == "" || newpath == "" {
		return fmt.Errorf("paths cannot be empty")
	}

	// Get obfuscated paths
	oldObfuscated, err := fs.getObfuscatedPath(oldpath)
	if err != nil {
		return fmt.Errorf("failed to get old obfuscated path: %w", err)
	}

	// For the new path, we need to handle the case where it might not exist yet
	newDir := filepath.Dir(newpath)
	if newDir == newpath {
		newDir = "."
	}
	newBaseName := filepath.Base(newpath)

	// Get obfuscated name for the new file
	newObfuscated, err := fs.obfuscateFilename(newDir, newBaseName)
	if err != nil {
		return fmt.Errorf("failed to obfuscate new filename: %w", err)
	}

	// Get the full obfuscated path for the new file
	newObfuscatedDir, err := fs.getObfuscatedPath(newDir)
	if err != nil {
		return fmt.Errorf("failed to get new obfuscated directory: %w", err)
	}

	newObfuscatedPath := filepath.Join(newObfuscatedDir, newObfuscated)

	// Perform the rename on the underlying filesystem
	if err := fs.underlying.Rename(oldObfuscated, newObfuscatedPath); err != nil {
		return err
	}

	// Update filemaps
	oldDir := filepath.Dir(oldpath)
	if oldDir == oldpath {
		oldDir = "."
	}

	// Remove from old filemap
	oldObfuscatedBase := filepath.Base(oldObfuscated)
	if err := fs.removeFromFilemap(oldDir, oldObfuscatedBase); err != nil {
		// Try to revert the rename if filemap update fails
		fs.underlying.Rename(newObfuscatedPath, oldObfuscated)
		return fmt.Errorf("failed to update old filemap: %w", err)
	}

	// Add to new filemap (this was already done in obfuscateFilename)
	return nil
}

// Remove removes a file
func (fs *GrainFS) Remove(filename string) error {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()

	if filename == "" {
		return fmt.Errorf("filename cannot be empty")
	}

	obfuscatedPath, err := fs.getObfuscatedPath(filename)
	if err != nil {
		return fmt.Errorf("failed to get obfuscated path: %w", err)
	}

	// Remove the file from underlying filesystem
	if err := fs.underlying.Remove(obfuscatedPath); err != nil {
		return err
	}

	// Update filemap
	dir := filepath.Dir(filename)
	if dir == filename {
		dir = "."
	}
	obfuscatedBase := filepath.Base(obfuscatedPath)

	return fs.removeFromFilemap(dir, obfuscatedBase)
}

// Join joins path elements
func (fs *GrainFS) Join(elem ...string) string {
	return filepath.Join(elem...)
}

// Dir interface implementation

// ReadDir reads the directory and returns file information
func (fs *GrainFS) ReadDir(path string) ([]os.FileInfo, error) {
	fs.mutex.RLock()
	defer fs.mutex.RUnlock()

	if path == "" {
		path = "."
	}

	obfuscatedPath, err := fs.getObfuscatedPath(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get obfuscated path: %w", err)
	}

	// Read the underlying directory
	infos, err := fs.underlying.ReadDir(obfuscatedPath)
	if err != nil {
		return nil, err
	}

	var result []os.FileInfo
	for _, info := range infos {
		// Skip .grainfs directories
		if info.Name() == GrainFSDir {
			continue
		}

		// Deobfuscate the filename
		originalName, err := fs.deobfuscateFilename(path, info.Name())
		if err != nil {
			// Skip files that can't be deobfuscated (might be corrupted)
			continue
		}

		// Wrap the FileInfo to show the original name
		wrappedInfo := &FileInfoWrapper{
			FileInfo:     info,
			originalName: originalName,
		}
		result = append(result, wrappedInfo)
	}

	return result, nil
}

// MkdirAll creates directories recursively
func (fs *GrainFS) MkdirAll(path string, perm os.FileMode) error {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()

	return fs.mkdirAllInternal(path, perm)
}

// mkdirAllInternal is the internal implementation that doesn't acquire locks
func (fs *GrainFS) mkdirAllInternal(path string, perm os.FileMode) error {
	if path == "" || path == "." {
		return nil
	}

	// We need to create directories step by step to ensure filemaps are created
	// Split the path and create each directory level
	parts := strings.Split(filepath.Clean(path), string(filepath.Separator))
	currentPath := ""

	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}

		if currentPath == "" {
			currentPath = part
		} else {
			currentPath = filepath.Join(currentPath, part)
		}

		// Get obfuscated path for this level
		obfuscatedPath, err := fs.getObfuscatedPath(currentPath)
		if err != nil {
			return fmt.Errorf("failed to get obfuscated path for %s: %w", currentPath, err)
		}

		// Create the directory if it doesn't exist
		if err := fs.underlying.MkdirAll(obfuscatedPath, perm); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", obfuscatedPath, err)
		}

		// Initialize .grainfs directory for this level
		if err := fs.ensureGrainFSDir(currentPath); err != nil {
			return fmt.Errorf("failed to ensure .grainfs directory for %s: %w", currentPath, err)
		}
	}

	return nil
}

// Symlink interface implementation

// Lstat returns file info without following symlinks
func (fs *GrainFS) Lstat(filename string) (os.FileInfo, error) {
	fs.mutex.RLock()
	defer fs.mutex.RUnlock()

	obfuscatedPath, err := fs.getObfuscatedPath(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to get obfuscated path: %w", err)
	}

	if symlinkFS, ok := fs.underlying.(billy.Symlink); ok {
		info, err := symlinkFS.Lstat(obfuscatedPath)
		if err != nil {
			return nil, err
		}
		return &FileInfoWrapper{
			FileInfo:     info,
			originalName: filepath.Base(filename),
		}, nil
	}

	// Fallback to regular Stat if Symlink interface not supported
	return fs.Stat(filename)
}

// Symlink creates a symbolic link
func (fs *GrainFS) Symlink(target, link string) error {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()

	symlinkFS, ok := fs.underlying.(billy.Symlink)
	if !ok {
		return fmt.Errorf("underlying filesystem does not support symlinks")
	}

	// Get obfuscated paths
	obfuscatedTarget, err := fs.getObfuscatedPath(target)
	if err != nil {
		return fmt.Errorf("failed to get obfuscated target path: %w", err)
	}

	obfuscatedLink, err := fs.getObfuscatedPath(link)
	if err != nil {
		return fmt.Errorf("failed to get obfuscated link path: %w", err)
	}

	return symlinkFS.Symlink(obfuscatedTarget, obfuscatedLink)
}

// Readlink reads the target of a symbolic link
func (fs *GrainFS) Readlink(link string) (string, error) {
	fs.mutex.RLock()
	defer fs.mutex.RUnlock()

	symlinkFS, ok := fs.underlying.(billy.Symlink)
	if !ok {
		return "", fmt.Errorf("underlying filesystem does not support symlinks")
	}

	obfuscatedLink, err := fs.getObfuscatedPath(link)
	if err != nil {
		return "", fmt.Errorf("failed to get obfuscated link path: %w", err)
	}

	obfuscatedTarget, err := symlinkFS.Readlink(obfuscatedLink)
	if err != nil {
		return "", err
	}

	// Convert back to user path
	return fs.getUserPath(obfuscatedTarget)
}

// Chroot interface implementation

// Chroot creates a new filesystem rooted at the given path
func (fs *GrainFS) Chroot(path string) (billy.Filesystem, error) {
	fs.mutex.RLock()
	defer fs.mutex.RUnlock()

	if path == "" {
		path = "."
	}

	obfuscatedPath, err := fs.getObfuscatedPath(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get obfuscated path: %w", err)
	}

	chrootFS, ok := fs.underlying.(billy.Chroot)
	if !ok {
		return nil, fmt.Errorf("underlying filesystem does not support chroot")
	}

	underlyingChroot, err := chrootFS.Chroot(obfuscatedPath)
	if err != nil {
		return nil, err
	}

	// Create a new GrainFS instance with the chrooted filesystem
	newFS := &GrainFS{
		underlying:  underlyingChroot,
		masterKey:   fs.masterKey,
		filenameKey: fs.filenameKey,
		rootPath:    filepath.Join(fs.rootPath, path),
	}
	newFS.filemapManager = NewFilemapManager(newFS)

	return newFS, nil
}

// Root returns the root path of the filesystem
func (fs *GrainFS) Root() string {
	return fs.rootPath
}

// TempFile interface implementation

// TempFile creates a temporary file
func (fs *GrainFS) TempFile(dir, prefix string) (billy.File, error) {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()

	tempFS, ok := fs.underlying.(billy.TempFile)
	if !ok {
		return nil, fmt.Errorf("underlying filesystem does not support temp files")
	}

	if dir == "" {
		dir = "."
	}

	obfuscatedDir, err := fs.getObfuscatedPath(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to get obfuscated directory: %w", err)
	}

	// Create temp file in underlying filesystem
	underlyingFile, err := tempFS.TempFile(obfuscatedDir, prefix)
	if err != nil {
		return nil, err
	}

	// Get the temp file's name and create mapping
	tempName := filepath.Base(underlyingFile.Name())

	// For temp files, we'll use a simple mapping without full obfuscation
	// since they're temporary and the name is generated by the system
	originalTempName := tempName // Keep the same name for temp files

	// Create encrypted file wrapper
	encFile := &EncryptedFile{
		underlying:  underlyingFile,
		fs:          fs,
		filename:    filepath.Join(dir, originalTempName),
		obfuscated:  underlyingFile.Name(),
		flag:        os.O_RDWR,
		isWriteMode: true,
		isTempFile:  true,
	}

	return encFile, nil
}

// FileInfoWrapper wraps os.FileInfo to show original filenames
type FileInfoWrapper struct {
	os.FileInfo
	originalName string
}

// Name returns the original filename
func (w *FileInfoWrapper) Name() string {
	return w.originalName
}
