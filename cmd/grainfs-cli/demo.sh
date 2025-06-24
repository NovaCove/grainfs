#!/bin/bash

# GrainFS CLI Demo Script
# This script demonstrates the capabilities of the GrainFS CLI

echo "=== GrainFS CLI Demo ==="
echo "This demo shows how to debug and navigate an encrypted filesystem"
echo

# Create a temporary demo directory
DEMO_DIR="./demo-temp"
rm -rf "$DEMO_DIR"
mkdir -p "$DEMO_DIR"

echo "1. Creating some test data..."
echo "Creating directories and files through the CLI..."

# Create test data using the CLI
./grainfs-cli "$DEMO_DIR" "demo-password" << 'EOF'
write README.md # GrainFS Demo

This is a demonstration of the encrypted filesystem.

mkdir documents
mkdir documents/private
mkdir documents/public
write documents/private/secret.txt This is highly confidential information!
write documents/private/passwords.txt admin:secret123
write documents/public/info.txt This is public information
mkdir projects
mkdir projects/project1
write projects/project1/notes.txt Project notes and ideas
ls
exit
EOF

echo
echo "2. Now let's explore the encrypted filesystem..."
echo

# Interactive exploration
./grainfs-cli "$DEMO_DIR" "demo-password" << 'EOF'
echo "=== Root Directory ==="
ls
echo
echo "=== Directory Tree ==="
tree
echo
echo "=== Raw Encrypted View ==="
raw
echo
echo "=== Navigate to documents ==="
cd documents
ls
echo
echo "=== Private documents ==="
cd private
ls
cat secret.txt
echo
echo "=== Back to root and check projects ==="
cd /
cd projects/project1
cat notes.txt
echo
echo "=== Debug information ==="
debug
exit
EOF

echo
echo "3. Comparison with raw filesystem:"
echo "Here's what the encrypted data looks like on disk:"
find "$DEMO_DIR" -type f -name "*.json" -o -name "*" | head -10

echo
echo "=== Demo Complete ==="
echo "The GrainFS CLI successfully demonstrated:"
echo "- Transparent file encryption/decryption"
echo "- Filename obfuscation"
echo "- Standard filesystem navigation"
echo "- Debug capabilities showing both views"
echo
echo "Clean up demo directory? (y/n)"
read -r response
if [[ "$response" =~ ^[Yy]$ ]]; then
    rm -rf "$DEMO_DIR"
    echo "Demo directory cleaned up."
fi
