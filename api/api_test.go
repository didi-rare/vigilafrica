package api

import "testing"

// TestAPIExists is a dummy test at the root of the api module.
// This ensures that 'go test ./...' from the root always has a valid test file
// to execute at the module level, preventing "no test files" warnings or closures in CI.
func TestAPIExists(t *testing.T) {
	t.Log("VigilAfrica API module detected and testable.")
}
