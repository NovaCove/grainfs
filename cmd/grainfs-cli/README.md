# GrainFS CLI

A simple shell CLI for debugging and navigating encrypted GrainFS filesystems.

## Features

- **Interactive Shell**: Navigate the encrypted filesystem like a regular shell
- **File Operations**: Read, write, create, and delete files and directories
- **Debug Mode**: View raw encrypted filesystem structure
- **Tree View**: Display directory structure in tree format
- **Dual View**: Compare encrypted vs decrypted filesystem views

## Installation

```bash
cd cmd/grainfs-cli
go build -o grainfs-cli
```

## Usage

```bash
./grainfs-cli <storage-path> [password]
```

- `storage-path`: Path to the encrypted filesystem storage directory
- `password`: Password for decryption (optional, will prompt if not provided)

## Example

```bash
# Using the demo data from the example
./grainfs-cli ../../example/demo my-secret-password-123
```

## Available Commands

### Navigation
- `ls, list [path]` - List files in directory
- `cd <path>` - Change current directory
- `pwd` - Print current directory
- `tree [path]` - Show directory tree

### File Operations
- `cat, read <file>` - Read and display file contents
- `write <file> <text>` - Write text to file
- `mkdir <path>` - Create directory
- `rm, remove <file>` - Remove file
- `stat <file>` - Show file information

### Debug Commands
- `debug [path]` - Show debug information
- `raw [path]` - Show raw encrypted filesystem contents
- `filemap [path]` - Show filename mappings (limited access)

### General
- `help, h` - Show help message
- `exit, quit, q` - Exit the CLI

## Example Session

```
$ ./grainfs-cli ../../example/demo my-secret-password-123
GrainFS CLI - Connected to: ../../example/demo
Type 'help' for available commands

grainfs:.> ls
Contents of .:
  dir       0  documents

grainfs:.> cd documents
grainfs:documents> ls
Contents of documents:
  dir       0  private

grainfs:documents> cd private
grainfs:documents/private> ls
Contents of documents/private:
  file     73  secret.txt

grainfs:documents/private> cat secret.txt
Contents of documents/private/secret.txt:
This is highly confidential information that will be encrypted!

grainfs:documents/private> raw
Raw encrypted filesystem contents (underlying storage):
[DIR]  .grainfs/
  [FILE] config.json (168 bytes)
  [FILE] filemap.json (108 bytes)
[DIR]  PyGtK8XChKUn2PCSeaCIqZZbTfWA381rIMZm2EOBb1AlY1W9aeFbtwKuKDipB7X4l9wXzVOoCFXd/
  [DIR]  .grainfs/
    [FILE] filemap.json (108 bytes)
  [DIR]  Fkx8LEkOCRhIjM67jEYu8w9p2hE4H7OTAdl48C4mAUtUF4U1lYRlRLdF1scbjPaNrR87P2TWww==/
    [DIR]  .grainfs/
      [FILE] filemap.json (108 bytes)
    [FILE] dUR0dVYon3ojezGE0tsxky8LB3ikls-ZrGtPCeUJJe74EIve_tUKxp_dcIPAyCNn7MCdWMuwWGsF-w== (105 bytes)

grainfs:documents/private> tree /
Directory tree for: .
└── documents/
    └── private/
        └── secret.txt

grainfs:documents/private> debug
Debug information for: documents/private
Storage root: ../../example/demo
Current path: documents/private

Encrypted filesystem structure:
Raw encrypted filesystem contents (underlying storage):
[DIR]  .grainfs/
  [FILE] config.json (168 bytes)
  [FILE] filemap.json (108 bytes)
[DIR]  PyGtK8XChKUn2PCSeaCIqZZbTfWA381rIMZm2EOBb1AlY1W9aeFbtwKuKDipB7X4l9wXzVOoCFXd/
  [DIR]  .grainfs/
    [FILE] filemap.json (108 bytes)
  [DIR]  Fkx8LEkOCRhIjM67jEYu8w9p2hE4H7OTAdl48C4mAUtUF4U1lYRlRLdF1scbjPaNrR87P2TWww==/
    [DIR]  .grainfs/
      [FILE] filemap.json (108 bytes)
    [FILE] dUR0dVYon3ojezGE0tsxky8LB3ikls-ZrGtPCeUJJe74EIve_tUKxp_dcIPAyCNn7MCdWMuwWGsF-w== (105 bytes)

grainfs:documents/private> exit
Goodbye!
```

## Key Features Demonstrated

1. **Transparent Decryption**: Files and directories appear with their original names
2. **Raw View**: The `raw` command shows the actual encrypted/obfuscated structure on disk
3. **Navigation**: Standard shell-like navigation with `cd`, `ls`, `pwd`
4. **File Operations**: Read and write files transparently
5. **Debug Information**: View both encrypted and decrypted views side by side

## Notes

- The CLI provides a user-friendly interface to explore encrypted GrainFS filesystems
- All file operations are performed through the GrainFS layer, ensuring proper encryption/decryption
- The `raw` command shows the underlying encrypted structure for debugging purposes
- Filename mappings are handled internally by GrainFS and are not directly accessible through the CLI
