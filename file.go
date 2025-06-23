package grainfs

import (
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/go-git/go-billy/v5"
)

// EncryptedFile wraps a billy.File to provide transparent encryption/decryption
type EncryptedFile struct {
	underlying  billy.File
	fs          *GrainFS
	filename    string
	obfuscated  string
	flag        int
	isWriteMode bool
	isTempFile  bool

	// For reading
	decryptingReader *DecryptingReader
	readInitialized  bool

	// For writing
	encryptingWriter *EncryptingWriter
	writeBuffer      []byte

	// Synchronization
	mutex  sync.RWMutex
	closed bool
}

// Ensure EncryptedFile implements billy.File
var _ billy.File = (*EncryptedFile)(nil)

// Read reads decrypted data from the file
func (f *EncryptedFile) Read(p []byte) (n int, err error) {
	f.mutex.RLock()
	defer f.mutex.RUnlock()

	if f.closed {
		return 0, os.ErrClosed
	}

	if f.isWriteMode {
		return 0, fmt.Errorf("file opened for writing")
	}

	// Initialize decrypting reader if not done yet
	if !f.readInitialized {
		if err := f.initializeReader(); err != nil {
			return 0, fmt.Errorf("failed to initialize reader: %w", err)
		}
		f.readInitialized = true
	}

	return f.decryptingReader.Read(p)
}

// ReadAt reads len(p) bytes from the file starting at byte offset off
func (f *EncryptedFile) ReadAt(p []byte, off int64) (n int, err error) {
	f.mutex.RLock()
	defer f.mutex.RUnlock()

	if f.closed {
		return 0, os.ErrClosed
	}

	if f.isWriteMode {
		return 0, fmt.Errorf("file opened for writing")
	}

	// For encrypted files, ReadAt is complex due to encryption overhead
	// We'll implement a simple version that reads the entire file and returns the requested portion
	if !f.readInitialized {
		if err := f.initializeReader(); err != nil {
			return 0, fmt.Errorf("failed to initialize reader: %w", err)
		}
		f.readInitialized = true
	}

	// Check bounds
	if off < 0 {
		return 0, fmt.Errorf("negative offset")
	}

	// Ensure the decrypting reader is initialized
	if f.decryptingReader == nil {
		return 0, fmt.Errorf("decrypting reader not initialized")
	}

	// Make sure the reader has been initialized (data decrypted)
	if !f.decryptingReader.initialized {
		return 0, fmt.Errorf("decrypting reader data not available")
	}

	if off >= int64(len(f.decryptingReader.decrypted)) {
		return 0, io.EOF
	}

	// Copy the requested portion
	available := int64(len(f.decryptingReader.decrypted)) - off
	n = len(p)
	if int64(n) > available {
		n = int(available)
		err = io.EOF
	}

	copy(p[:n], f.decryptingReader.decrypted[off:off+int64(n)])
	return n, err
}

// Write encrypts and writes data to the file
func (f *EncryptedFile) Write(p []byte) (n int, err error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if f.closed {
		return 0, os.ErrClosed
	}

	if !f.isWriteMode {
		return 0, fmt.Errorf("file not opened for writing")
	}

	// Initialize encrypting writer if not done yet
	if f.encryptingWriter == nil {
		var err error
		f.encryptingWriter, err = NewEncryptingWriter(f.underlying, f.fs.masterKey)
		if err != nil {
			return 0, fmt.Errorf("failed to initialize encrypting writer: %w", err)
		}
	}

	return f.encryptingWriter.Write(p)
}

// Close closes the file and finalizes encryption if writing
func (f *EncryptedFile) Close() error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if f.closed {
		return nil
	}

	var err error

	// Finalize encryption if we were writing
	if f.encryptingWriter != nil {
		if closeErr := f.encryptingWriter.Close(); closeErr != nil {
			err = fmt.Errorf("failed to finalize encryption: %w", closeErr)
		}
	}

	// Close underlying file
	if closeErr := f.underlying.Close(); closeErr != nil {
		if err == nil {
			err = closeErr
		}
	}

	f.closed = true
	return err
}

// Seek sets the file position for the next read or write
func (f *EncryptedFile) Seek(offset int64, whence int) (int64, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if f.closed {
		return 0, os.ErrClosed
	}

	// For encrypted files, seeking is complex because of the encryption overhead
	// For now, we'll support limited seeking scenarios

	if f.isWriteMode {
		// For write mode, we can only seek to the beginning before any writes
		if f.encryptingWriter != nil {
			return 0, fmt.Errorf("seeking not supported after writing to encrypted file")
		}

		if offset == 0 && whence == io.SeekStart {
			// Seeking to start is OK before writing
			return f.underlying.Seek(offset, whence)
		}

		return 0, fmt.Errorf("seeking not supported in write mode for encrypted files")
	}

	// For read mode, we need to handle the encryption overhead
	if offset == 0 && whence == io.SeekStart {
		// Reset to beginning - reinitialize reader
		f.readInitialized = false
		f.decryptingReader = nil

		// Seek underlying file to start
		pos, err := f.underlying.Seek(0, io.SeekStart)
		if err != nil {
			return 0, err
		}

		return pos, nil
	}

	// Other seek operations are not supported for encrypted files
	return 0, fmt.Errorf("seeking not fully supported for encrypted files")
}

// Name returns the filename
func (f *EncryptedFile) Name() string {
	return f.filename
}

// Truncate truncates the file to the specified size
func (f *EncryptedFile) Truncate(size int64) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if f.closed {
		return os.ErrClosed
	}

	if !f.isWriteMode {
		return fmt.Errorf("file not opened for writing")
	}

	// For encrypted files, truncation is complex
	// We'll support truncating to 0 (clearing the file)
	if size == 0 {
		// Reset the file
		if f.encryptingWriter != nil {
			f.encryptingWriter = nil
		}

		// Truncate underlying file to 0
		if err := f.underlying.Truncate(0); err != nil {
			return err
		}

		// Seek to beginning
		_, err := f.underlying.Seek(0, io.SeekStart)
		return err
	}

	return fmt.Errorf("truncation to non-zero size not supported for encrypted files")
}

// Lock locks the file (if supported by underlying filesystem)
func (f *EncryptedFile) Lock() error {
	type locker interface {
		Lock() error
	}
	if l, ok := f.underlying.(locker); ok {
		return l.Lock()
	}
	return fmt.Errorf("file locking not supported by underlying filesystem")
}

// Unlock unlocks the file (if supported by underlying filesystem)
func (f *EncryptedFile) Unlock() error {
	type unlocker interface {
		Unlock() error
	}
	if u, ok := f.underlying.(unlocker); ok {
		return u.Unlock()
	}
	return fmt.Errorf("file unlocking not supported by underlying filesystem")
}

// initializeReader sets up the decrypting reader and reads all data
func (f *EncryptedFile) initializeReader() error {
	// Seek to beginning of file
	if _, err := f.underlying.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek to beginning: %w", err)
	}

	// Create decrypting reader
	var err error
	f.decryptingReader, err = NewDecryptingReader(f.underlying, f.fs.masterKey)
	if err != nil {
		return fmt.Errorf("failed to create decrypting reader: %w", err)
	}

	// Force initialization by reading a byte (this triggers the initialize method)
	// We'll read and then reset the position
	_, err = f.decryptingReader.Read(make([]byte, 1))
	if err != nil && err != io.EOF {
		return fmt.Errorf("failed to initialize decrypting reader: %w", err)
	}

	// Reset position to beginning
	f.decryptingReader.pos = 0

	return nil
}

// Stat returns file information
func (f *EncryptedFile) Stat() (os.FileInfo, error) {
	f.mutex.RLock()
	defer f.mutex.RUnlock()

	if f.closed {
		return nil, os.ErrClosed
	}

	// Use the filesystem's Stat method instead of the file's
	info, err := f.fs.underlying.Stat(f.obfuscated)
	if err != nil {
		return nil, err
	}

	// For encrypted files, we need to adjust the size to account for encryption overhead
	// The actual decrypted size is smaller than the encrypted size on disk

	// If we have a decrypting reader that's been initialized, we can get the actual size
	if f.decryptingReader != nil && f.decryptingReader.initialized {
		actualSize := int64(len(f.decryptingReader.decrypted))
		return &EncryptedFileInfo{
			FileInfo:     info,
			actualSize:   actualSize,
			originalName: f.filename,
		}, nil
	}

	// If we haven't read the file yet, we can't determine the actual size
	// Return the encrypted size for now
	return &EncryptedFileInfo{
		FileInfo:     info,
		actualSize:   info.Size(),
		originalName: f.filename,
	}, nil
}

// EncryptedFileInfo wraps os.FileInfo for encrypted files
type EncryptedFileInfo struct {
	os.FileInfo
	actualSize   int64
	originalName string
}

// Size returns the actual decrypted size
func (efi *EncryptedFileInfo) Size() int64 {
	return efi.actualSize
}

// Name returns the original filename
func (efi *EncryptedFileInfo) Name() string {
	return efi.originalName
}
