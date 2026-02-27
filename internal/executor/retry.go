package executor

import "fmt"

// BuildRetryPrompt creates the prompt for a retry attempt.
// attempt is 0-indexed retry number (1 = first retry, etc.)
// maxRetries is the maximum number of retries configured.
func BuildRetryPrompt(attempt, maxRetries int, testOutput string) string {
	totalAttempts := 1 + maxRetries
	currentAttempt := attempt + 1

	prompt := fmt.Sprintf("The previous implementation attempt failed. This is attempt %d of %d.\n", currentAttempt, totalAttempts)

	if attempt == maxRetries {
		prompt += "This is your final attempt — focus on the most critical fix.\n"
	}

	prompt += "\nTEST OUTPUT:\n"
	prompt += TruncateTestOutput(testOutput, 4000)
	prompt += "\n\nPlease fix the failing tests. Focus on:\n"
	prompt += "1. Read the error messages carefully\n"
	prompt += "2. Identify the root cause\n"
	prompt += "3. Make the minimum change needed to fix the issue\n"
	prompt += "4. Run the tests again to verify\n"

	return prompt
}

// TruncateTestOutput trims test output to maxChars, keeping the
// beginning and end (the most useful parts). Inserts a truncation
// notice in the middle.
func TruncateTestOutput(output string, maxChars int) string {
	if len(output) <= maxChars {
		return output
	}

	// Keep first 40% and last 40%
	keepEach := maxChars * 2 / 5
	truncated := len(output) - keepEach*2
	if truncated < 0 {
		return output
	}

	return output[:keepEach] +
		fmt.Sprintf("\n\n[... %d chars truncated ...]\n\n", truncated) +
		output[len(output)-keepEach:]
}
