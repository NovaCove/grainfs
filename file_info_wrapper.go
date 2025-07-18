package grainfs

import "os"

// FileInfoWrapper wraps os.FileInfo to show original filenames
type FileInfoWrapper struct {
	os.FileInfo
	originalName string
}

// Name returns the original filename
func (w *FileInfoWrapper) Name() string {
	return w.originalName
}
