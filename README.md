# GrainFS: Encrypted Billy Filesystem

**WARNING: THIS IS NOT STABLE - USE AT YOUR OWN RISK**

GrainFS is a transparent encryption layer that implements the `billy.Filesystem` interface from `github.com/go-git/go-billy/v5`. It provides seamless file content encryption and filename obfuscation while maintaining full compatibility with the billy ecosystem.

## Features

- **Full Billy Interface Compatibility**: Implements all billy interfaces (`Basic`, `Dir`, `Symlink`, `Chroot`, `TempFile`)
- **Strong Encryption**: AES-256-GCM for file content encryption with unique nonces
- **Filename Obfuscation**: AES-256-CTR with HMAC-SHA256 for secure filename encryption
- **Key Derivation**: PBKDF2 with SHA-256 (100,000 iterations) for secure key generation
- **Transparent Operation**: Works as a drop-in replacement for any billy filesystem
- **Concurrent Safe**: Thread-safe operations with proper synchronization
- **Streaming Support**: Efficient handling of large files

## Architecture

### Encryption Details

**File Content Encryption:**
- Algorithm: AES-256-GCM
- Nonce: 96-bit random nonce per file
- Format: `[nonce][encrypted_data][auth_tag]`
- Authentication: Built-in authentication with GCM mode

**Filename Obfuscation:**
- Algorithm: AES-256-CTR + HMAC-SHA256
- Encoding: Base64url for filesystem compatibility
- Collision Handling: Automatic counter suffixes
- Maximum Length: 200 characters

**Key Derivation:**
- Master Key: PBKDF2-SHA256 with 100,000 iterations
- Filename Key: Derived from master key with different salt
- Salt Storage: Stored in `.grainfs/config.json`

### Directory Structure

```
underlying_filesystem/
├── .grainfs/
│   ├── config.json          # Encrypted configuration
│   └── filemap.json         # Encrypted filename mappings
├── obfuscated_filename_1    # Encrypted file
├── obfuscated_filename_2    # Encrypted file
└── obfuscated_dir_name/     # Obfuscated directory
    ├── .grainfs/
    │   └── filemap.json     # Directory-specific mappings
    └── obfuscated_file      # Encrypted file in subdirectory
```

## Installation

```bash
go get github.com/NovaCove/grainfs
```

## Usage

### Basic Example

```go
package main

import (
    "fmt"
    "io"
    "log"

    "github.com/NovaCove/grainfs"
    "github.com/go-git/go-billy/v5/osfs"
)

func main() {
    // Create underlying filesystem
    underlying := osfs.New("/path/to/storage")
    
    // Create encrypted filesystem
    fs, err := grainfs.New(underlying, "my-secret-password")
    if err != nil {
        log.Fatal(err)
    }

    // Use like any billy.Filesystem
    file, err := fs.Create("secret.txt")
    if err != nil {
        log.Fatal(err)
    }
    defer file.Close()

    // Write encrypted data
    _, err = file.Write([]byte("sensitive information"))
    if err != nil {
        log.Fatal(err)
    }

    // Read decrypted data
    file, err = fs.Open("secret.txt")
    if err != nil {
        log.Fatal(err)
    }
    defer file.Close()

    data, err := io.ReadAll(file)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Decrypted: %s\n", data)
}
```

### Advanced Usage

```go
// Directory operations
err := fs.MkdirAll("documents/private", 0755)

// File operations with flags
file, err := fs.OpenFile("data.txt", os.O_RDWR|os.O_CREATE, 0644)

// Directory listing
infos, err := fs.ReadDir("documents")

// File operations
err = fs.Rename("old.txt", "new.txt")
err = fs.Remove("unwanted.txt")

// Chroot for sandboxing
subFS, err := fs.Chroot("documents")
```

## API Reference

### Core Types

```go
type GrainFS struct {
    // Internal fields
}

func New(underlying billy.Filesystem, password string) (*GrainFS, error)
```

### Supported Interfaces

- `billy.Filesystem` - Core filesystem operations
- `billy.Basic` - Basic file operations (Create, Open, OpenFile, Stat, Rename, Remove, Join)
- `billy.Dir` - Directory operations (ReadDir, MkdirAll)
- `billy.Symlink` - Symbolic link operations (Lstat, Symlink, Readlink)
- `billy.Chroot` - Chroot operations (Chroot, Root)
- `billy.TempFile` - Temporary file operations (TempFile)

### File Operations

All standard billy file operations are supported:

```go
// File creation and access
file, err := fs.Create(filename)
file, err := fs.Open(filename)
file, err := fs.OpenFile(filename, flag, perm)

// File information
info, err := fs.Stat(filename)

// File manipulation
err := fs.Rename(oldpath, newpath)
err := fs.Remove(filename)

// Directory operations
infos, err := fs.ReadDir(path)
err := fs.MkdirAll(path, perm)
```

## Security Considerations

### Encryption Security

- **AES-256-GCM**: Provides both confidentiality and authenticity
- **Unique Nonces**: Each file write uses a cryptographically random nonce
- **Key Derivation**: PBKDF2 with high iteration count protects against brute force
- **Authenticated Encryption**: Prevents tampering with encrypted data

### Filename Security

- **Deterministic Obfuscation**: Same filename always produces same obfuscated result
- **HMAC Authentication**: Prevents filename tampering
- **Collision Resistance**: Automatic handling of hash collisions
- **Length Limits**: Prevents filesystem compatibility issues

### Operational Security

- **Memory Safety**: Keys are zeroed when possible
- **Error Handling**: No information leakage through error messages
- **Constant Time**: Filename comparisons use constant-time operations
- **Atomic Operations**: Filemap updates are atomic to prevent corruption

## Performance

### Benchmarks

```
BenchmarkGrainFSWrite-12    161834    7566 ns/op
BenchmarkGrainFSRead-12     520732    2363 ns/op
```

### Performance Characteristics

- **Write Performance**: ~7.5μs per 1KB write operation
- **Read Performance**: ~2.4μs per 1KB read operation
- **Memory Usage**: Minimal overhead, streaming encryption/decryption
- **Scalability**: Concurrent operations supported

## Limitations

### Current Limitations

1. **Seek Operations**: Limited seeking support for encrypted files
2. **Truncation**: Only truncation to zero size supported
3. **ReadAt Performance**: Requires full file decryption for random access
4. **File Size**: Encrypted files have small overhead (nonce + auth tag)

### Future Improvements

- Block-based encryption for better random access
- Streaming ReadAt implementation
- Optimized seek operations
- Compression support

## Testing

Run the comprehensive test suite:

```bash
# Run all tests
go test -v

# Run specific tests
go test -run TestGrainFSBasicOperations
go test -run TestGrainFSEncryption

# Run benchmarks
go test -bench=.

# Test with real filesystem
go test -run TestGrainFSWithOSFS
```

## Examples

See the `example/` directory for complete usage examples:

```bash
cd example
go run main.go
```

## Compatibility

### Billy Ecosystem

GrainFS is fully compatible with the billy ecosystem:

- **go-git**: Can be used as storage backend for Git repositories
- **Billy utilities**: Works with all billy-compatible tools
- **Filesystem adapters**: Compatible with osfs, memfs, and other billy implementations

### Go Version

- Requires Go 1.19 or later
- Uses `golang.org/x/crypto` for cryptographic operations

## Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure all tests pass
5. Submit a pull request

## License
MIT

## Security Disclosure

For security issues, please email security@novacove.ai instead of using the issue tracker.

