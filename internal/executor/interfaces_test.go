package executor

import "testing"

// Verify mocks satisfy interfaces at compile time.
func TestMocksSatisfyInterfaces(t *testing.T) {
	t.Parallel()
	var _ GitOps = (*MockGitOps)(nil)
	var _ TestRunner = (*MockTestRunner)(nil)
	var _ ClaudeExecutor = (*MockClaudeExecutor)(nil)
	var _ GitOps = (*RealGitOps)(nil)
	var _ TestRunner = (*RealTestRunner)(nil)
	var _ ClaudeExecutor = (*RealClaudeExecutor)(nil)
}
