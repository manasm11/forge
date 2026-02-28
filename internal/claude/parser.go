package claude

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

// PlanJSON represents the structured plan output from Claude during initial planning.
type PlanJSON struct {
	ProjectName string         `json:"project_name"`
	Description string         `json:"description"`
	TechStack   []string       `json:"tech_stack"`
	Tasks       []PlanTaskJSON `json:"tasks"`
}

// PlanTaskJSON represents a single task in the initial plan.
type PlanTaskJSON struct {
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
	DependsOn          []int    `json:"depends_on,omitempty"`
	Complexity         string   `json:"estimated_complexity"`
}

// PlanUpdateJSON represents the structured output from a replanning session.
type PlanUpdateJSON struct {
	Summary string               `json:"summary"`
	Tasks   []PlanUpdateTaskJSON `json:"tasks"`
}

// PlanUpdateTaskJSON represents a single task action in a plan update.
type PlanUpdateTaskJSON struct {
	ID                 string   `json:"id,omitempty"`
	Action             string   `json:"action"`
	Title              string   `json:"title,omitempty"`
	Description        string   `json:"description,omitempty"`
	AcceptanceCriteria []string `json:"acceptance_criteria,omitempty"`
	DependsOn          []string `json:"depends_on,omitempty"`
	Complexity         string   `json:"estimated_complexity,omitempty"`
	Reason             string   `json:"reason,omitempty"`
}

// parseResponse parses the raw JSON output from claude --output-format json.
func parseResponse(raw []byte) (*Response, error) {
	if len(raw) == 0 {
		return &Response{}, nil
	}

	var jsonResp map[string]interface{}
	if err := json.Unmarshal(raw, &jsonResp); err != nil {
		// Not valid JSON — treat the entire output as plain text
		return &Response{
			Text:    string(raw),
			RawJSON: string(raw),
		}, nil
	}

	resp := &Response{
		RawJSON: string(raw),
	}

	// Extract text — try common field names
	if result, ok := jsonResp["result"].(string); ok {
		resp.Text = result
	} else if text, ok := jsonResp["text"].(string); ok {
		resp.Text = text
	} else {
		resp.Text = string(raw)
	}

	// Extract session ID if present
	if sid, ok := jsonResp["session_id"].(string); ok {
		resp.SessionID = sid
	}

	return resp, nil
}

// ExtractFinalPlan checks if the response text contains <final_plan>...</final_plan> tags.
// If found, parses the JSON inside and returns the plan.
// Returns nil, nil if no tags found.
// Returns nil, error if tags found but JSON is malformed or missing required fields.
func ExtractFinalPlan(text string) (*PlanJSON, error) {
	content, found := extractTagContent(text, "final_plan")
	if !found {
		return nil, nil
	}

	var plan PlanJSON
	if err := json.Unmarshal([]byte(content), &plan); err != nil {
		return nil, fmt.Errorf("invalid JSON in <final_plan>: %w", err)
	}

	if plan.ProjectName == "" {
		return nil, fmt.Errorf("invalid plan: missing project_name")
	}
	if len(plan.Tasks) == 0 {
		return nil, fmt.Errorf("invalid plan: at least one task is required")
	}

	return &plan, nil
}

// ExtractPlanUpdate checks if the response text contains <plan_update>...</plan_update> tags.
// If found, parses the JSON inside and returns the update.
// Returns nil, nil if no tags found.
// Returns nil, error if tags found but JSON is malformed or invalid.
func ExtractPlanUpdate(text string) (*PlanUpdateJSON, error) {
	content, found := extractTagContent(text, "plan_update")
	if !found {
		return nil, nil
	}

	var update PlanUpdateJSON
	if err := json.Unmarshal([]byte(content), &update); err != nil {
		return nil, fmt.Errorf("invalid JSON in <plan_update>: %w", err)
	}

	for i, task := range update.Tasks {
		if task.Action == "" {
			return nil, fmt.Errorf("invalid plan update: task %d is missing action field", i)
		}
	}

	return &update, nil
}

// parseStreamChunk extracts displayable text from a single line of stream-json output.
// Returns empty string if the line doesn't contain displayable text.
// Must handle unknown/unexpected JSON structures gracefully.
func parseStreamChunk(line string) string {
	if line == "" {
		return ""
	}

	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(line), &obj); err != nil {
		// Not JSON — might be raw text output. Return as-is if non-empty.
		return strings.TrimSpace(line)
	}

	msgType, _ := obj["type"].(string)

	// content_block_delta with text_delta
	if delta, ok := obj["delta"].(map[string]interface{}); ok {
		if text, ok := delta["text"].(string); ok {
			return text
		}
	}

	// assistant message with content array (initial message)
	if msg, ok := obj["message"].(map[string]interface{}); ok {
		if content, ok := msg["content"].([]interface{}); ok {
			var texts []string
			for _, block := range content {
				if b, ok := block.(map[string]interface{}); ok {
					if text, ok := b["text"].(string); ok {
						texts = append(texts, text)
					}
				}
			}
			return strings.Join(texts, "")
		}
	}

	// result type — full assembled text, skip to avoid duplication
	if msgType == "result" {
		return ""
	}

	// content_block_start with initial text
	if cb, ok := obj["content_block"].(map[string]interface{}); ok {
		if text, ok := cb["text"].(string); ok && text != "" {
			return text
		}
	}

	// Log unknown types for debugging (only non-control types)
	if msgType != "" && msgType != "message_start" && msgType != "message_stop" &&
		msgType != "content_block_start" && msgType != "content_block_stop" &&
		msgType != "ping" && msgType != "message_delta" &&
		msgType != "system_limit_event" && msgType != "rate_limit_event" {
		log.Printf("unknown stream-json type: %s", msgType)
	}

	return ""
}

// extractTagContent extracts content between <tag>...</tag>.
// Returns the content and true if found, empty string and false if not.
func extractTagContent(text, tag string) (string, bool) {
	openTag := "<" + tag + ">"
	closeTag := "</" + tag + ">"

	openIdx := strings.Index(text, openTag)
	if openIdx == -1 {
		return "", false
	}

	contentStart := openIdx + len(openTag)
	closeIdx := strings.Index(text[contentStart:], closeTag)
	if closeIdx == -1 {
		return "", false
	}

	content := text[contentStart : contentStart+closeIdx]
	return strings.TrimSpace(content), true
}
