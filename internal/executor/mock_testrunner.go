package executor

import "context"

// MockTestRunner returns predefined test results.
type MockTestRunner struct {
	Results []*TestResult
	Calls   []string // commands that were run
	callIdx int
}

var _ TestRunner = (*MockTestRunner)(nil)

// NewMockTestRunner creates a mock that returns the given results in order.
func NewMockTestRunner(results ...*TestResult) *MockTestRunner {
	return &MockTestRunner{Results: results}
}

func (m *MockTestRunner) RunTests(ctx context.Context, command string) *TestResult {
	m.Calls = append(m.Calls, command)
	if m.callIdx < len(m.Results) {
		r := m.Results[m.callIdx]
		m.callIdx++
		return r
	}
	return &TestResult{Passed: true, Output: "ok"}
}

func (m *MockTestRunner) RunBuild(ctx context.Context, command string) *TestResult {
	m.Calls = append(m.Calls, command)
	if m.callIdx < len(m.Results) {
		r := m.Results[m.callIdx]
		m.callIdx++
		return r
	}
	return &TestResult{Passed: true, Output: "ok"}
}

// MockClaudeExecutor returns predefined execution results.
type MockClaudeExecutor struct {
	Results []*ExecuteResult
	Errors  []error
	Calls   []ExecuteOpts
	callIdx int
}

var _ ClaudeExecutor = (*MockClaudeExecutor)(nil)

// NewMockClaudeExecutor creates a mock that returns the given results in order.
func NewMockClaudeExecutor(results ...*ExecuteResult) *MockClaudeExecutor {
	errs := make([]error, len(results))
	return &MockClaudeExecutor{Results: results, Errors: errs}
}

func (m *MockClaudeExecutor) Execute(ctx context.Context, opts ExecuteOpts) (*ExecuteResult, error) {
	m.Calls = append(m.Calls, opts)
	if m.callIdx < len(m.Results) {
		r := m.Results[m.callIdx]
		e := m.Errors[m.callIdx]
		m.callIdx++

		// Simulate streaming if chunk callback provided
		if opts.OnChunk != nil && r != nil {
			opts.OnChunk(r.Text)
		}
		return r, e
	}
	return &ExecuteResult{Text: "done"}, nil
}
