package executor

import "context"

// MockGitOps records all calls and returns predefined results.
type MockGitOps struct {
	CurrentBranchResult string
	CurrentBranchErr    error

	CreateBranchCalls []string // branch names
	CreateBranchErr   error

	CheckoutCalls []string
	CheckoutErr   error

	BranchExistsResult map[string]bool

	StageAllCalls int
	StageAllErr   error

	HasStagedResult bool
	HasStagedErr    error

	HasUnstagedResult bool

	CommitCalls []string // commit messages
	CommitSHA   string   // SHA to return
	CommitErr   error

	PushCalls int
	PushErr   error

	LatestSHAResult string
	LatestSHAErr    error

	ResetHardCalls int
	ResetHardErr   error

	DeleteBranchCalls []string
}

var _ GitOps = (*MockGitOps)(nil)

// NewMockGitOps returns a mock with sensible defaults.
func NewMockGitOps() *MockGitOps {
	return &MockGitOps{
		CurrentBranchResult: "main",
		BranchExistsResult:  make(map[string]bool),
		CommitSHA:           "abc123def456",
		HasStagedResult:     true,
	}
}

func (m *MockGitOps) CurrentBranch(ctx context.Context) (string, error) {
	return m.CurrentBranchResult, m.CurrentBranchErr
}

func (m *MockGitOps) CreateBranch(ctx context.Context, name, base string) error {
	m.CreateBranchCalls = append(m.CreateBranchCalls, name)
	return m.CreateBranchErr
}

func (m *MockGitOps) CheckoutBranch(ctx context.Context, name string) error {
	m.CheckoutCalls = append(m.CheckoutCalls, name)
	return m.CheckoutErr
}

func (m *MockGitOps) BranchExists(ctx context.Context, name string) (bool, error) {
	return m.BranchExistsResult[name], nil
}

func (m *MockGitOps) StageAll(ctx context.Context) error {
	m.StageAllCalls++
	return m.StageAllErr
}

func (m *MockGitOps) HasStagedChanges(ctx context.Context) (bool, error) {
	return m.HasStagedResult, m.HasStagedErr
}

func (m *MockGitOps) HasUnstagedChanges(ctx context.Context) (bool, error) {
	return m.HasUnstagedResult, nil
}

func (m *MockGitOps) Commit(ctx context.Context, message string) (string, error) {
	m.CommitCalls = append(m.CommitCalls, message)
	return m.CommitSHA, m.CommitErr
}

func (m *MockGitOps) Push(ctx context.Context) error {
	m.PushCalls++
	return m.PushErr
}

func (m *MockGitOps) LatestSHA(ctx context.Context) (string, error) {
	return m.LatestSHAResult, m.LatestSHAErr
}

func (m *MockGitOps) ResetHard(ctx context.Context) error {
	m.ResetHardCalls++
	return m.ResetHardErr
}

func (m *MockGitOps) DeleteBranch(ctx context.Context, name string) error {
	m.DeleteBranchCalls = append(m.DeleteBranchCalls, name)
	return nil
}
