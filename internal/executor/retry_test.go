package executor

import (
	"strings"
	"testing"
)

func TestBuildRetryPrompt(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		attempt     int
		maxRetries  int
		testOutput  string
		mustContain []string
	}{
		{
			name:       "first retry",
			attempt:    1,
			maxRetries: 3,
			testOutput: "FAIL TestAuth: expected 200, got 401",
			mustContain: []string{
				"attempt 2 of 4",
				"FAIL TestAuth",
				"expected 200, got 401",
				"fix the failing test",
			},
		},
		{
			name:       "last retry",
			attempt:    3,
			maxRetries: 3,
			testOutput: "FAIL: build error",
			mustContain: []string{
				"attempt 4 of 4",
				"final attempt",
				"FAIL: build error",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			prompt := BuildRetryPrompt(tt.attempt, tt.maxRetries, tt.testOutput)
			for _, s := range tt.mustContain {
				if !strings.Contains(prompt, s) {
					t.Errorf("retry prompt missing %q\ngot:\n%s", s, prompt)
				}
			}
		})
	}
}

func TestTruncateTestOutput(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		output    string
		maxChars  int
		wantTrunc bool
	}{
		{
			name:      "short output unchanged",
			output:    "PASS ok",
			maxChars:  500,
			wantTrunc: false,
		},
		{
			name:      "long output truncated from middle",
			output:    longString(2000),
			maxChars:  500,
			wantTrunc: true,
		},
		{
			name:      "empty output",
			output:    "",
			maxChars:  500,
			wantTrunc: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := TruncateTestOutput(tt.output, tt.maxChars)
			if tt.wantTrunc {
				if len(result) > tt.maxChars+100 {
					t.Errorf("output not truncated: len = %d", len(result))
				}
				if !strings.Contains(result, "truncated") {
					t.Error("truncated output should contain truncation notice")
				}
			}
			if !tt.wantTrunc && tt.output != "" && result != tt.output {
				t.Errorf("short output should be unchanged")
			}
			if tt.output == "" && result != "" {
				t.Errorf("empty output should remain empty")
			}
		})
	}
}

func longString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'x'
	}
	return string(b)
}
