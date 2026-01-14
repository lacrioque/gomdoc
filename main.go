// Package main is the entry point for the gomdoc application.
// gomdoc is a simple markdown server that renders .md files as HTML.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"gomdoc/server"
)

func main() {
	port := flag.Int("port", 7331, "Port to run the server on")
	dir := flag.String("dir", ".", "Base directory to serve markdown files from")
	flag.Parse()

	// Resolve and validate the base directory
	baseDir, err := filepath.Abs(*dir)
	if err != nil {
		log.Fatalf("Error resolving directory path: %v", err)
	}

	info, err := os.Stat(baseDir)
	if err != nil {
		log.Fatalf("Error accessing directory: %v", err)
	}
	if !info.IsDir() {
		log.Fatalf("Path is not a directory: %s", baseDir)
	}

	fmt.Println("gomdoc - Markdown Documentation Server")
	fmt.Println("=======================================")

	srv := server.New(baseDir, *port)
	if err := srv.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
