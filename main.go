// Package main is the entry point for the gomdoc application.
// gomdoc is a simple markdown server that renders .md files as HTML.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gomdoc/server"
)

func main() {
	port := flag.Int("port", 7331, "Port to run the server on")
	dir := flag.String("dir", ".", "Base directory to serve markdown files from")
	title := flag.String("title", "gomdoc", "Custom title for the documentation site")
	auth := flag.String("auth", "", "Basic auth credentials in user:password format")
	flag.Parse()

	// Validate auth format if provided
	var authUser, authPass string
	if *auth != "" {
		parts := strings.SplitN(*auth, ":", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			log.Fatalf("Invalid auth format. Use: -auth user:password")
		}
		authUser = parts[0]
		authPass = parts[1]
	}

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

	srv := server.New(baseDir, *port, *title, authUser, authPass)
	if err := srv.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
