module grainfs-cli

go 1.24.3

require (
	github.com/NovaCove/grainfs v0.0.0
	github.com/go-git/go-billy/v5 v5.6.2
)

require (
	github.com/cyphar/filepath-securejoin v0.3.6 // indirect
	golang.org/x/crypto v0.31.0 // indirect
	golang.org/x/sys v0.29.0 // indirect
)

replace github.com/NovaCove/grainfs => ../..
