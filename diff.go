package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
)

// diffLocalAndRemote compares local project files with files inside the deployed container
func (r *remote) diffLocalAndRemote(name string) error {
	return withSSHClient(r.address, r.config, func(client *ssh.Client) error {
		fmt.Println("comparing local files with deployed container...")

		// load ignore patterns from .dockerignore and parse Dockerfile
		ignorePatterns := loadDockerIgnorePatterns()
		dockerfileIncludes := parseDockerfileCopyPatterns()

		if len(ignorePatterns) > 0 {
			fmt.Printf("loaded %d patterns from .dockerignore\n", len(ignorePatterns))
		}
		if len(dockerfileIncludes) > 0 {
			fmt.Printf("found %d COPY/ADD patterns in Dockerfile\n", len(dockerfileIncludes))
		}

		// check if container is running
		_, _, err := runSSHCommand(client, fmt.Sprintf("sudo docker inspect %s", name), "")
		if err != nil {
			return fmt.Errorf("container %s is not running or doesn't exist", name)
		}

		// get the working directory of the container
		workdir, _, err := runSSHCommand(client,
			fmt.Sprintf("sudo docker inspect -f '{{.Config.WorkingDir}}' %s", name), "")
		if err != nil {
			return fmt.Errorf("failed to get container working directory: %v", err)
		}
		workdir = strings.TrimSpace(workdir)
		if workdir == "" {
			workdir = "/" // default to root if no workdir set
		}

		fmt.Printf("container working directory: %s\n", workdir)

		// get list of local files
		localFiles := make(map[string]string)
		err = filepath.Walk(".", func(path string, info fs.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// skip directories
			if info.IsDir() {
				return nil
			}

			// skip files based on dockerignore, Dockerfile patterns, and default patterns
			if shouldSkipFile(path, ignorePatterns) {
				return nil
			}

			// if Dockerfile has specific COPY patterns, only include matching files
			if len(dockerfileIncludes) > 0 && !matchesAnyDockerfilePattern(path, dockerfileIncludes) {
				return nil
			}

			// store relative path
			localFiles[path] = path
			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to list local files: %v", err)
		}

		// get list of files from container
		remoteFiles := make(map[string]string)
		err = getContainerFiles(client, name, workdir, remoteFiles, ignorePatterns)
		if err != nil {
			return fmt.Errorf("failed to list container files: %v", err)
		}

		// find files that exist in both places and have differences
		fmt.Println("\n=== Files with differences ===")
		filesWithDiffs := []string{}

		for localPath := range localFiles {
			remoteRelPath := localPath
			if _, exists := remoteFiles[remoteRelPath]; exists {
				// compare the files
				isDifferent, err := compareLocalWithContainer(client, name, localPath, workdir+"/"+remoteRelPath)
				if err != nil {
					fmt.Printf("warning: failed to compare %s: %v\n", localPath, err)
					continue
				}

				if isDifferent {
					filesWithDiffs = append(filesWithDiffs, localPath)
					fmt.Printf("  - %s\n", localPath)
				}
			}
		}

		// find files only in local
		fmt.Println("\n=== Files only in local (new files) ===")
		onlyLocal := []string{}
		for localPath := range localFiles {
			remoteRelPath := localPath
			if _, exists := remoteFiles[remoteRelPath]; !exists {
				onlyLocal = append(onlyLocal, localPath)
				fmt.Printf("  + %s\n", localPath)
			}
		}

		// find files only in remote
		fmt.Println("\n=== Files only in remote (deleted files) ===")
		onlyRemote := []string{}
		for remotePath := range remoteFiles {
			if _, exists := localFiles[remotePath]; !exists {
				onlyRemote = append(onlyRemote, remotePath)
				fmt.Printf("  - %s\n", remotePath)
			}
		}

		// show detailed diffs for modified files
		if len(filesWithDiffs) > 0 {
			fmt.Println("\n=== Detailed differences ===")
			for _, filePath := range filesWithDiffs {
				err := showContainerFileDiff(client, name, filePath, workdir+"/"+filePath)
				if err != nil {
					fmt.Printf("warning: failed to show diff for %s: %v\n", filePath, err)
				}
			}
		}

		// summary
		fmt.Println("\n=== Summary ===")
		fmt.Printf("Modified files: %d\n", len(filesWithDiffs))
		fmt.Printf("New files (local only): %d\n", len(onlyLocal))
		fmt.Printf("Deleted files (remote only): %d\n", len(onlyRemote))

		return nil
	})
}

// loadDockerIgnorePatterns reads .dockerignore and returns patterns
func loadDockerIgnorePatterns() []string {
	patterns := []string{}

	content, err := os.ReadFile(".dockerignore")
	if err != nil {
		return patterns // file doesn't exist, return empty
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}

	return patterns
}

// parseDockerfileCopyPatterns extracts COPY/ADD source patterns from Dockerfile
func parseDockerfileCopyPatterns() []string {
	patterns := []string{}

	content, err := os.ReadFile("Dockerfile")
	if err != nil {
		return patterns // file doesn't exist, return empty
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// look for COPY or ADD commands
		if strings.HasPrefix(line, "COPY ") || strings.HasPrefix(line, "ADD ") {
			// parse the COPY/ADD line
			// format: COPY [--from=stage] <src>... <dest>
			parts := strings.Fields(line)
			if len(parts) < 3 {
				continue
			}

			// skip flags like --from=builder
			startIdx := 1
			for startIdx < len(parts) && strings.HasPrefix(parts[startIdx], "--") {
				startIdx++
			}

			// get all source patterns (everything except the last part which is dest)
			for i := startIdx; i < len(parts)-1; i++ {
				src := parts[i]
				// normalize the pattern
				src = strings.TrimPrefix(src, "./")
				if src != "" && src != "." {
					patterns = append(patterns, src)
				} else if src == "." {
					// COPY . /app means copy everything
					return []string{} // empty means "copy all"
				}
			}
		}
	}

	return patterns
}

// matchesAnyDockerfilePattern checks if a path matches any Dockerfile COPY pattern
func matchesAnyDockerfilePattern(path string, patterns []string) bool {
	path = strings.TrimPrefix(path, "./")

	for _, pattern := range patterns {
		// exact match
		if path == pattern {
			return true
		}

		// directory pattern - if pattern is a dir, match all files under it
		if strings.HasSuffix(pattern, "/") {
			if strings.HasPrefix(path, pattern) {
				return true
			}
		}

		// check if pattern is a directory (no extension, no wildcards)
		if !strings.Contains(pattern, "*") && !strings.Contains(pattern, ".") {
			if strings.HasPrefix(path, pattern+"/") || path == pattern {
				return true
			}
		}

		// glob match
		if matched, _ := filepath.Match(pattern, path); matched {
			return true
		}

		// check if path starts with pattern (for directory matches)
		if strings.HasPrefix(path, pattern+"/") {
			return true
		}
	}

	return false
}

// shouldSkipFile determines if a file should be skipped during comparison
func shouldSkipFile(path string, dockerignorePatterns []string) bool {
	// always skip these lord-specific and common build artifact files
	alwaysSkip := []string{
		// lord-specific
		".git/",
		".gitignore",
		"lord.yml",
		".lord.yml",
		"Dockerfile",
		".dockerignore",
		"CLAUDE.md",
		"*.tar.gz",
		"lord-logs/",
		"lord",

		// common build artifacts and cache directories
		"__pycache__/",
		"*.pyc",
		"*.pyo",
		"*.pyd",
		".Python",
		"node_modules/",
		"npm-debug.log",
		"yarn-error.log",
		".npm",
		".yarn",
		"vendor/",
		"target/",
		"build/",
		"dist/",
		"*.egg-info/",
		".pytest_cache/",
		".coverage",
		".env",
		".venv/",
		"venv/",
		"env/",
		".DS_Store",
		"Thumbs.db",
		"*.swp",
		"*.swo",
		"*~",
		".idea/",
		".vscode/",
		"*.log",
	}

	// check always-skip patterns
	for _, pattern := range alwaysSkip {
		if matchesPattern(path, pattern) {
			return true
		}
	}

	// check dockerignore patterns
	for _, pattern := range dockerignorePatterns {
		if matchesPattern(path, pattern) {
			return true
		}
	}

	return false
}

// matchesPattern checks if a path matches a dockerignore-style pattern
func matchesPattern(path, pattern string) bool {
	// normalize path separators
	path = filepath.ToSlash(path)
	pattern = filepath.ToSlash(pattern)

	// remove leading ./
	path = strings.TrimPrefix(path, "./")

	// exact match
	if path == pattern {
		return true
	}

	// directory match (pattern ends with /)
	if strings.HasSuffix(pattern, "/") {
		if strings.HasPrefix(path, pattern) || strings.Contains(path, "/"+strings.TrimSuffix(pattern, "/")) {
			return true
		}
	}

	// contains check for simple patterns
	if strings.Contains(path, pattern) {
		return true
	}

	// glob pattern match
	matched, _ := filepath.Match(pattern, filepath.Base(path))
	if matched {
		return true
	}

	// match against full path
	matched, _ = filepath.Match(pattern, path)
	return matched
}

// getContainerFiles lists all files in the container's working directory
func getContainerFiles(client *ssh.Client, containerName, workdir string, files map[string]string, ignorePatterns []string) error {
	// use find to list all files in the container
	cmd := fmt.Sprintf("sudo docker exec %s find %s -type f 2>/dev/null || true", containerName, workdir)
	output, _, err := runSSHCommandSilent(client, cmd, "")
	if err != nil {
		return err
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		// get relative path from workdir
		relPath := strings.TrimPrefix(line, workdir+"/")
		if relPath == line {
			continue // skip if it's not under workdir
		}

		if !shouldSkipFile(relPath, ignorePatterns) {
			files[relPath] = line
		}
	}

	return nil
}

// compareLocalWithContainer compares a local file with a file inside the container
func compareLocalWithContainer(client *ssh.Client, containerName, localPath, containerPath string) (bool, error) {
	// read local file
	localContent, err := os.ReadFile(localPath)
	if err != nil {
		return false, err
	}

	// read file from container using docker exec
	cmd := fmt.Sprintf("sudo docker exec %s cat %s 2>/dev/null", containerName, containerPath)
	containerContent, _, err := runSSHCommandSilent(client, cmd, "")
	if err != nil {
		return false, fmt.Errorf("failed to read container file: %v", err)
	}

	// compare byte-by-byte
	return string(localContent) != containerContent, nil
}

// showContainerFileDiff shows a unified diff between local and container files
func showContainerFileDiff(client *ssh.Client, containerName, localPath, containerPath string) error {
	// read local file
	localContent, err := os.ReadFile(localPath)
	if err != nil {
		return err
	}

	// read file from container
	cmd := fmt.Sprintf("sudo docker exec %s cat %s 2>/dev/null", containerName, containerPath)
	containerContent, _, err := runSSHCommandSilent(client, cmd, "")
	if err != nil {
		return fmt.Errorf("failed to read container file: %v", err)
	}

	fmt.Printf("\n--- a/%s (container)\n", localPath)
	fmt.Printf("+++ b/%s (local)\n", localPath)

	// split into lines
	containerLines := strings.Split(containerContent, "\n")
	localLines := strings.Split(string(localContent), "\n")

	// simple line-by-line diff
	diff := generateUnifiedDiff(containerLines, localLines)

	for _, line := range diff {
		fmt.Println(line)
	}

	return nil
}

// generateUnifiedDiff creates a unified diff output
func generateUnifiedDiff(oldLines, newLines []string) []string {
	var result []string

	maxLen := len(oldLines)
	if len(newLines) > maxLen {
		maxLen = len(newLines)
	}

	// track chunks of changes
	var chunkStart int = -1
	var chunkLines []string
	contextLines := 3 // number of context lines to show

	for i := 0; i < maxLen; i++ {
		oldLine := ""
		newLine := ""

		if i < len(oldLines) {
			oldLine = oldLines[i]
		}
		if i < len(newLines) {
			newLine = newLines[i]
		}

		if oldLine != newLine {
			// start a new chunk if needed
			if chunkStart == -1 {
				chunkStart = max(0, i-contextLines)
				// add context lines before the change
				for j := chunkStart; j < i; j++ {
					if j < len(oldLines) {
						chunkLines = append(chunkLines, " "+oldLines[j])
					}
				}
			}

			// add the changed lines
			if i < len(oldLines) && i < len(newLines) {
				// line modified
				chunkLines = append(chunkLines, "-"+oldLine)
				chunkLines = append(chunkLines, "+"+newLine)
			} else if i >= len(oldLines) {
				// line added
				chunkLines = append(chunkLines, "+"+newLine)
			} else {
				// line deleted
				chunkLines = append(chunkLines, "-"+oldLine)
			}
		} else if chunkStart != -1 {
			// we're in a chunk, add as context
			chunkLines = append(chunkLines, " "+oldLine)

			// check if we should close the chunk
			if shouldCloseChunk(i, oldLines, newLines, contextLines) {
				// output the chunk with header
				result = append(result, fmt.Sprintf("@@ -%d,%d +%d,%d @@",
					chunkStart+1, len(chunkLines), chunkStart+1, len(chunkLines)))
				result = append(result, chunkLines...)
				chunkStart = -1
				chunkLines = nil
			}
		}
	}

	// flush any remaining chunk
	if chunkStart != -1 {
		result = append(result, fmt.Sprintf("@@ -%d,%d +%d,%d @@",
			chunkStart+1, len(chunkLines), chunkStart+1, len(chunkLines)))
		result = append(result, chunkLines...)
	}

	return result
}

// shouldCloseChunk determines if we should close the current diff chunk
func shouldCloseChunk(currentIdx int, oldLines, newLines []string, contextLines int) bool {
	// look ahead to see if there are more changes coming
	checkAhead := min(contextLines, 5)
	for i := 1; i <= checkAhead; i++ {
		idx := currentIdx + i
		if idx >= len(oldLines) && idx >= len(newLines) {
			return true
		}

		oldLine := ""
		newLine := ""
		if idx < len(oldLines) {
			oldLine = oldLines[idx]
		}
		if idx < len(newLines) {
			newLine = newLines[idx]
		}

		if oldLine != newLine {
			return false // more changes ahead, don't close yet
		}
	}
	return true
}

// helper functions
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
