package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	baseBranch     = "origin/development"
	changeDir      = "openspec/changes/"
	trivialFlag    = "[trivial]"
)

var criticalPaths = []string{
	"api/internal/",
	"api/cmd/",
	"web/src/",
}

var allowList = []string{
	"api/db/migrations/",
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "\n🛡️  Sentinel Audit Failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("\n✅ Sentinel Audit Passed: Governance requirements satisfied.")
}

func run() error {
	fmt.Printf("🔍 Starting VigilAfrica Sentinel Audit against %s...\n", baseBranch)

	// 1. Check for [trivial] in commit messages
	isTrivial, err := checkTrivial()
	if err != nil {
		return fmt.Errorf("failed to check commit history: %w", err)
	}
	if isTrivial {
		fmt.Println("ℹ️  Trivial bypass detected in commit history. Skipping deep audit.")
		return nil
	}

	// 2. Get diff
	files, err := getDiffFiles()
	if err != nil {
		return fmt.Errorf("failed to get git diff: %w", err)
	}

	criticalChanges := []string{}
	governanceChanges := []string{}

	for _, file := range files {
		if strings.HasPrefix(file, changeDir) && !strings.Contains(file, "/archive/") {
			governanceChanges = append(governanceChanges, file)
			continue
		}

		if isCritical(file) && !isAllowed(file) {
			criticalChanges = append(criticalChanges, file)
		}
	}

	fmt.Printf("📊 Audit results: %d critical code changes, %d governance records.\n", len(criticalChanges), len(governanceChanges))

	if len(criticalChanges) > 0 && len(governanceChanges) == 0 {
		fmt.Println("\n❌ GHOST IMPLEMENTATION DETECTED")
		fmt.Println("You have modified critical source code without proposing an OpenSpec change.")
		fmt.Println("Modified files:")
		for _, f := range criticalChanges {
			fmt.Printf("  - %s\n", f)
		}
		fmt.Println("\n👉 SOLUTION: Run '/opsx-propose' to register this feature before implementing,")
		fmt.Println("   or add '[trivial]' to your commit message if this is a minor maintenance task.")
		return fmt.Errorf("governance violation")
	}

	return nil
}

func getDiffFiles() ([]string, error) {
	// Triple dot diff finds changes in current branch since it diverged from base
	cmd := exec.Command("git", "diff", baseBranch+"...HEAD", "--name-only")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	
	lines := strings.Split(out.String(), "\n")
	var files []string
	for _, l := range lines {
		if t := strings.TrimSpace(l); t != "" {
			files = append(files, t)
		}
	}
	return files, nil
}

func checkTrivial() (bool, error) {
	cmd := exec.Command("git", "log", baseBranch+"..HEAD", "--format=%B")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		// If baseBranch isn't found locally, the diff logic might fail.
		// In CI we ensure origin/development is fetched.
		return false, nil 
	}

	return strings.Contains(strings.ToLower(out.String()), trivialFlag), nil
}

func isCritical(path string) bool {
	for _, p := range criticalPaths {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

func isAllowed(path string) bool {
	for _, p := range allowList {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}
