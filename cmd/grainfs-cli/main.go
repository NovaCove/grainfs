package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/NovaCove/grainfs"
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
)

type CLI struct {
	fs          *grainfs.GrainFS
	underlying  billy.Filesystem
	currentPath string
	password    string
	rootPath    string
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: grainfs-cli <storage-path> [password]")
		fmt.Println("  storage-path: Path to the encrypted filesystem storage")
		fmt.Println("  password:     Password for decryption (will prompt if not provided)")
		os.Exit(1)
	}

	storagePath := os.Args[1]
	var password string

	if len(os.Args) >= 3 {
		password = os.Args[2]
	} else {
		fmt.Print("Enter password: ")
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			password = scanner.Text()
		}
		if password == "" {
			fmt.Println("Password cannot be empty")
			os.Exit(1)
		}
	}

	// Create underlying filesystem
	underlying := osfs.New(storagePath)

	// Create GrainFS
	fs, err := grainfs.New(underlying, password)
	if err != nil {
		fmt.Printf("Failed to initialize GrainFS: %v\n", err)
		os.Exit(1)
	}

	cli := &CLI{
		fs:          fs,
		underlying:  underlying,
		currentPath: ".",
		password:    password,
		rootPath:    storagePath,
	}

	fmt.Printf("GrainFS CLI - Connected to: %s\n", storagePath)
	fmt.Println("Type 'help' for available commands")

	cli.run()
}

func (c *CLI) run() {
	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Printf("grainfs:%s> ", c.currentPath)

		if !scanner.Scan() {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}

		command := parts[0]
		args := parts[1:]

		switch command {
		case "help", "h":
			c.showHelp()
		case "ls", "list":
			c.listFiles(args)
		case "cd":
			c.changeDirectory(args)
		case "pwd":
			c.printWorkingDirectory()
		case "cat", "read":
			c.readFile(args)
		case "write":
			c.writeFile(args)
		case "mkdir":
			c.makeDirectory(args)
		case "rm", "remove":
			c.removeFile(args)
		case "stat":
			c.statFile(args)
		case "debug":
			c.debugInfo(args)
		case "raw":
			c.showRawFiles(args)
		case "filemap":
			c.showFilemap(args)
		case "tree":
			c.showTree(args)
		case "exit", "quit", "q":
			fmt.Println("Goodbye!")
			return
		default:
			fmt.Printf("Unknown command: %s\n", command)
			fmt.Println("Type 'help' for available commands")
		}
	}
}

func (c *CLI) showHelp() {
	fmt.Println("Available commands:")
	fmt.Println("  help, h              - Show this help message")
	fmt.Println("  ls, list [path]      - List files in directory")
	fmt.Println("  cd <path>            - Change current directory")
	fmt.Println("  pwd                  - Print current directory")
	fmt.Println("  cat, read <file>     - Read and display file contents")
	fmt.Println("  write <file> <text>  - Write text to file")
	fmt.Println("  mkdir <path>         - Create directory")
	fmt.Println("  rm, remove <file>    - Remove file")
	fmt.Println("  stat <file>          - Show file information")
	fmt.Println("  debug [path]         - Show debug information")
	fmt.Println("  raw [path]           - Show raw encrypted filesystem contents")
	fmt.Println("  filemap [path]       - Show filename mappings")
	fmt.Println("  tree [path]          - Show directory tree")
	fmt.Println("  exit, quit, q        - Exit the CLI")
}

func (c *CLI) listFiles(args []string) {
	path := c.currentPath
	if len(args) > 0 {
		path = c.resolvePath(args[0])
	}

	infos, err := c.fs.ReadDir(path)
	if err != nil {
		fmt.Printf("Error listing directory: %v\n", err)
		return
	}

	if len(infos) == 0 {
		fmt.Println("Directory is empty")
		return
	}

	// Sort by name
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Name() < infos[j].Name()
	})

	fmt.Printf("Contents of %s:\n", path)
	for _, info := range infos {
		typeStr := "file"
		if info.IsDir() {
			typeStr = "dir "
		}
		fmt.Printf("  %s  %8d  %s\n", typeStr, info.Size(), info.Name())
	}
}

func (c *CLI) changeDirectory(args []string) {
	if len(args) == 0 {
		c.currentPath = "."
		return
	}

	newPath := c.resolvePath(args[0])

	// Check if directory exists
	info, err := c.fs.Stat(newPath)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	if !info.IsDir() {
		fmt.Printf("Error: %s is not a directory\n", newPath)
		return
	}

	c.currentPath = newPath
}

func (c *CLI) printWorkingDirectory() {
	fmt.Println(c.currentPath)
}

func (c *CLI) readFile(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: cat <filename>")
		return
	}

	filename := c.resolvePath(args[0])

	file, err := c.fs.Open(filename)
	if err != nil {
		fmt.Printf("Error opening file: %v\n", err)
		return
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		return
	}

	fmt.Printf("Contents of %s:\n", filename)
	fmt.Println(string(content))
}

func (c *CLI) writeFile(args []string) {
	if len(args) < 2 {
		fmt.Println("Usage: write <filename> <text>")
		return
	}

	filename := c.resolvePath(args[0])
	text := strings.Join(args[1:], " ")

	file, err := c.fs.Create(filename)
	if err != nil {
		fmt.Printf("Error creating file: %v\n", err)
		return
	}
	defer file.Close()

	_, err = file.Write([]byte(text))
	if err != nil {
		fmt.Printf("Error writing to file: %v\n", err)
		return
	}

	fmt.Printf("Successfully wrote to %s\n", filename)
}

func (c *CLI) makeDirectory(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: mkdir <directory>")
		return
	}

	dirPath := c.resolvePath(args[0])

	err := c.fs.MkdirAll(dirPath, 0755)
	if err != nil {
		fmt.Printf("Error creating directory: %v\n", err)
		return
	}

	fmt.Printf("Successfully created directory: %s\n", dirPath)
}

func (c *CLI) removeFile(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: rm <filename>")
		return
	}

	filename := c.resolvePath(args[0])

	err := c.fs.Remove(filename)
	if err != nil {
		fmt.Printf("Error removing file: %v\n", err)
		return
	}

	fmt.Printf("Successfully removed: %s\n", filename)
}

func (c *CLI) statFile(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: stat <filename>")
		return
	}

	filename := c.resolvePath(args[0])

	info, err := c.fs.Stat(filename)
	if err != nil {
		fmt.Printf("Error getting file info: %v\n", err)
		return
	}

	fmt.Printf("File: %s\n", filename)
	fmt.Printf("  Name: %s\n", info.Name())
	fmt.Printf("  Size: %d bytes\n", info.Size())
	fmt.Printf("  Mode: %s\n", info.Mode())
	fmt.Printf("  ModTime: %s\n", info.ModTime())
	fmt.Printf("  IsDir: %v\n", info.IsDir())
}

func (c *CLI) debugInfo(args []string) {
	path := c.currentPath
	if len(args) > 0 {
		path = c.resolvePath(args[0])
	}

	fmt.Printf("Debug information for: %s\n", path)
	fmt.Printf("Storage root: %s\n", c.rootPath)
	fmt.Printf("Current path: %s\n", c.currentPath)

	// Try to get obfuscated path using reflection or internal access
	// Since we can't access private methods directly, we'll show what we can
	fmt.Printf("\nEncrypted filesystem structure:\n")
	c.showRawFiles([]string{})
}

func (c *CLI) showRawFiles(args []string) {
	path := "."
	if len(args) > 0 {
		path = args[0]
	}

	fmt.Printf("Raw encrypted filesystem contents (underlying storage):\n")
	c.showRawDirectory(path, 0)
}

func (c *CLI) showRawDirectory(path string, depth int) {
	indent := strings.Repeat("  ", depth)

	infos, err := c.underlying.ReadDir(path)
	if err != nil {
		fmt.Printf("%sError reading directory: %v\n", indent, err)
		return
	}

	for _, info := range infos {
		if info.IsDir() {
			fmt.Printf("%s[DIR]  %s/\n", indent, info.Name())
			if depth < 3 { // Limit recursion depth
				subPath := filepath.Join(path, info.Name())
				c.showRawDirectory(subPath, depth+1)
			}
		} else {
			fmt.Printf("%s[FILE] %s (%d bytes)\n", indent, info.Name(), info.Size())
		}
	}
}

func (c *CLI) showFilemap(args []string) {
	path := c.currentPath
	if len(args) > 0 {
		path = c.resolvePath(args[0])
	}

	fmt.Printf("Filename mappings for directory: %s\n", path)
	fmt.Println("(Note: This requires access to internal GrainFS methods)")
	fmt.Println("Use 'raw' command to see the encrypted filenames on disk")
}

func (c *CLI) showTree(args []string) {
	path := c.currentPath
	if len(args) > 0 {
		path = c.resolvePath(args[0])
	}

	fmt.Printf("Directory tree for: %s\n", path)
	c.showTreeRecursive(path, "", true)
}

func (c *CLI) showTreeRecursive(path, prefix string, isLast bool) {
	infos, err := c.fs.ReadDir(path)
	if err != nil {
		fmt.Printf("%sError: %v\n", prefix, err)
		return
	}

	// Sort directories first, then files
	sort.Slice(infos, func(i, j int) bool {
		if infos[i].IsDir() != infos[j].IsDir() {
			return infos[i].IsDir()
		}
		return infos[i].Name() < infos[j].Name()
	})

	for i, info := range infos {
		isLastItem := i == len(infos)-1

		var connector string
		if isLastItem {
			connector = "└── "
		} else {
			connector = "├── "
		}

		typeIndicator := ""
		if info.IsDir() {
			typeIndicator = "/"
		}

		fmt.Printf("%s%s%s%s\n", prefix, connector, info.Name(), typeIndicator)

		if info.IsDir() {
			var newPrefix string
			if isLastItem {
				newPrefix = prefix + "    "
			} else {
				newPrefix = prefix + "│   "
			}

			subPath := filepath.Join(path, info.Name())
			c.showTreeRecursive(subPath, newPrefix, isLastItem)
		}
	}
}

func (c *CLI) resolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}

	if path == "." {
		return c.currentPath
	}

	if path == ".." {
		if c.currentPath == "." {
			return "."
		}
		return filepath.Dir(c.currentPath)
	}

	if strings.HasPrefix(path, "../") {
		// Handle relative paths with ..
		parts := strings.Split(path, "/")
		currentParts := strings.Split(c.currentPath, "/")

		for _, part := range parts {
			if part == ".." {
				if len(currentParts) > 1 {
					currentParts = currentParts[:len(currentParts)-1]
				}
			} else if part != "." && part != "" {
				currentParts = append(currentParts, part)
			}
		}

		result := strings.Join(currentParts, "/")
		if result == "" {
			return "."
		}
		return result
	}

	if c.currentPath == "." {
		return path
	}

	return filepath.Join(c.currentPath, path)
}
