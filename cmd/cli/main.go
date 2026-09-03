package main

import (
	"os"

	"go-hephaestus/internal/cli"
)

func main() {
	cli.RunCLI(os.Args[1:])
}
