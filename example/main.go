package main

import (
	"fmt"
	"io"
	"log"

	"github.com/NovaCove/grainfs"
	"github.com/go-git/go-billy/v5/osfs"
)

func main() {
	// Create an underlying filesystem (using memory filesystem for demo)
	// underlying := memfs.New()
	underlying := osfs.New("./demo")

	// Create GrainFS with encryption
	password := "my-secret-password-123"
	fs, err := grainfs.New(underlying, password)
	if err != nil {
		log.Fatalf("Failed to create GrainFS: %v", err)
	}

	fmt.Println("=== GrainFS Demo ===")

	// Create a directory
	fmt.Println("\n1. Creating directory structure...")
	err = fs.MkdirAll("documents/private", 0755)
	if err != nil {
		log.Fatalf("Failed to create directory: %v", err)
	}

	// Create and write to a file
	fmt.Println("2. Creating and writing to encrypted file...")
	file, err := fs.Create("documents/private/secret.txt")
	if err != nil {
		log.Fatalf("Failed to create file: %v", err)
	}

	secretData := "This is highly confidential information that will be encrypted!"
	_, err = file.Write([]byte(secretData))
	if err != nil {
		log.Fatalf("Failed to write to file: %v", err)
	}
	file.Close()

	// Read the file back
	fmt.Println("3. Reading encrypted file...")
	file, err = fs.Open("documents/private/secret.txt")
	if err != nil {
		log.Fatalf("Failed to open file: %v", err)
	}

	readData, err := io.ReadAll(file)
	if err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}
	file.Close()

	fmt.Printf("   Original: %s\n", secretData)
	fmt.Printf("   Read back: %s\n", string(readData))
	fmt.Printf("   Match: %v\n", secretData == string(readData))

	// List directory contents
	fmt.Println("\n4. Listing directory contents...")
	infos, err := fs.ReadDir("documents/private")
	if err != nil {
		log.Fatalf("Failed to read directory: %v", err)
	}

	for _, info := range infos {
		fmt.Printf("   File: %s (size: %d bytes)\n", info.Name(), info.Size())
	}

	// Demonstrate that underlying filesystem has encrypted/obfuscated data
	fmt.Println("\n5. Checking underlying filesystem (encrypted/obfuscated)...")
	underlyingInfos, err := underlying.ReadDir(".")
	if err != nil {
		log.Fatalf("Failed to read underlying directory: %v", err)
	}

	fmt.Println("   Underlying filesystem contents:")
	for _, info := range underlyingInfos {
		fmt.Printf("   - %s (dir: %v)\n", info.Name(), info.IsDir())
	}

	// Try to read raw encrypted file
	fmt.Println("\n6. Attempting to read raw encrypted data...")
	// Find the obfuscated directory
	var obfuscatedDir string
	for _, info := range underlyingInfos {
		if info.IsDir() && info.Name() != ".grainfs" {
			obfuscatedDir = info.Name()
			break
		}
	}

	if obfuscatedDir != "" {
		subInfos, err := underlying.ReadDir(obfuscatedDir)
		if err == nil {
			for _, info := range subInfos {
				if !info.IsDir() && info.Name() != ".grainfs" {
					rawFile, err := underlying.Open(obfuscatedDir + "/" + info.Name())
					if err == nil {
						rawData, err := io.ReadAll(rawFile)
						rawFile.Close()
						if err == nil {
							fmt.Printf("   Raw encrypted data (first 50 bytes): %x...\n", rawData[:min(50, len(rawData))])
							fmt.Printf("   Raw data is different from original: %v\n", string(rawData) != secretData)
						}
					}
					break
				}
			}
		}
	}

	fmt.Println("\n=== Demo Complete ===")
	fmt.Println("GrainFS successfully encrypted file contents and obfuscated filenames!")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
