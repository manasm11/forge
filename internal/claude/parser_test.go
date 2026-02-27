package claude

import (
	"testing"
)

func TestExtractTagContent(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		tag       string
		wantText  string
		wantFound bool
	}{
		{
			name:      "valid tags with JSON content",
			text:      `<final_plan>{"key": "val"}</final_plan>`,
			tag:       "final_plan",
			wantText:  `{"key": "val"}`,
			wantFound: true,
		},
		{
			name: "multiline content",
			text: "<final_plan>\n{\n  \"key\": \"val\"\n}\n</final_plan>",
			tag:       "final_plan",
			wantText:  "{\n  \"key\": \"val\"\n}",
			wantFound: true,
		},
		{
			name:      "no tags present",
			text:      "Here is some regular text without any tags.",
			tag:       "final_plan",
			wantFound: false,
		},
		{
			name:      "opening tag but no closing tag",
			text:      `<final_plan>{"key": "val"}`,
			tag:       "final_plan",
			wantFound: false,
		},
		{
			name:      "extra content around tags",
			text:      `Here's the plan:\n<final_plan>{"key": "val"}</final_plan>\nLet me know!`,
			tag:       "final_plan",
			wantText:  `{"key": "val"}`,
			wantFound: true,
		},
		{
			name:      "empty content between tags",
			text:      `<final_plan></final_plan>`,
			tag:       "final_plan",
			wantText:  "",
			wantFound: true,
		},
		{
			name:      "plan_update tag",
			text:      `<plan_update>{"summary": "changes"}</plan_update>`,
			tag:       "plan_update",
			wantText:  `{"summary": "changes"}`,
			wantFound: true,
		},
		{
			name:      "content with angle brackets inside",
			text:      `<final_plan>{"desc": "use <b>bold</b> text"}</final_plan>`,
			tag:       "final_plan",
			wantText:  `{"desc": "use <b>bold</b> text"}`,
			wantFound: true,
		},
		{
			name:      "wrong tag name",
			text:      `<final_plan>content</final_plan>`,
			tag:       "plan_update",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found := extractTagContent(tt.text, tt.tag)
			if found != tt.wantFound {
				t.Errorf("found = %v, want %v", found, tt.wantFound)
			}
			if found && got != tt.wantText {
				t.Errorf("content = %q, want %q", got, tt.wantText)
			}
		})
	}
}

func TestExtractFinalPlan(t *testing.T) {
	t.Run("valid plan with all fields", func(t *testing.T) {
		text := `<final_plan>
{
  "project_name": "my-api",
  "description": "A REST API",
  "tech_stack": ["Go", "PostgreSQL"],
  "tasks": [
    {
      "title": "Init project",
      "description": "Set up Go module",
      "acceptance_criteria": ["go.mod exists", "go build passes"],
      "depends_on": [],
      "estimated_complexity": "small"
    },
    {
      "title": "Add auth",
      "description": "JWT authentication",
      "acceptance_criteria": ["login endpoint works"],
      "depends_on": [0],
      "estimated_complexity": "medium"
    }
  ]
}
</final_plan>`

		plan, err := ExtractFinalPlan(text)
		if err != nil {
			t.Fatalf("ExtractFinalPlan() error: %v", err)
		}
		if plan == nil {
			t.Fatal("plan should not be nil")
		}
		if plan.ProjectName != "my-api" {
			t.Errorf("ProjectName = %q, want %q", plan.ProjectName, "my-api")
		}
		if plan.Description != "A REST API" {
			t.Errorf("Description = %q, want %q", plan.Description, "A REST API")
		}
		if len(plan.TechStack) != 2 {
			t.Errorf("TechStack length = %d, want 2", len(plan.TechStack))
		}
		if len(plan.Tasks) != 2 {
			t.Fatalf("Tasks length = %d, want 2", len(plan.Tasks))
		}
		if plan.Tasks[0].Title != "Init project" {
			t.Errorf("Tasks[0].Title = %q, want %q", plan.Tasks[0].Title, "Init project")
		}
		if plan.Tasks[0].Complexity != "small" {
			t.Errorf("Tasks[0].Complexity = %q, want %q", plan.Tasks[0].Complexity, "small")
		}
		if len(plan.Tasks[0].AcceptanceCriteria) != 2 {
			t.Errorf("Tasks[0].AcceptanceCriteria length = %d, want 2", len(plan.Tasks[0].AcceptanceCriteria))
		}
		if len(plan.Tasks[1].DependsOn) != 1 || plan.Tasks[1].DependsOn[0] != 0 {
			t.Errorf("Tasks[1].DependsOn = %v, want [0]", plan.Tasks[1].DependsOn)
		}
	})

	t.Run("minimal valid plan", func(t *testing.T) {
		text := `<final_plan>
{
  "project_name": "test",
  "tasks": [
    {
      "title": "Setup",
      "description": "Init",
      "acceptance_criteria": ["works"],
      "estimated_complexity": "small"
    }
  ]
}
</final_plan>`

		plan, err := ExtractFinalPlan(text)
		if err != nil {
			t.Fatalf("ExtractFinalPlan() error: %v", err)
		}
		if plan == nil {
			t.Fatal("plan should not be nil")
		}
		if plan.ProjectName != "test" {
			t.Errorf("ProjectName = %q, want %q", plan.ProjectName, "test")
		}
	})

	t.Run("no tags — returns nil nil", func(t *testing.T) {
		text := "Just some text without any plan tags."
		plan, err := ExtractFinalPlan(text)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if plan != nil {
			t.Error("plan should be nil when no tags present")
		}
	})

	t.Run("tags with invalid JSON", func(t *testing.T) {
		text := `<final_plan>not valid json{{{</final_plan>`
		plan, err := ExtractFinalPlan(text)
		if err == nil {
			t.Fatal("should return error for invalid JSON")
		}
		if plan != nil {
			t.Error("plan should be nil on error")
		}
	})

	t.Run("missing project_name", func(t *testing.T) {
		text := `<final_plan>
{
  "tasks": [{"title": "t", "description": "d", "acceptance_criteria": ["a"], "estimated_complexity": "small"}]
}
</final_plan>`
		plan, err := ExtractFinalPlan(text)
		if err == nil {
			t.Fatal("should return error for missing project_name")
		}
		if plan != nil {
			t.Error("plan should be nil on error")
		}
	})

	t.Run("missing tasks", func(t *testing.T) {
		text := `<final_plan>{"project_name": "test", "tasks": []}</final_plan>`
		plan, err := ExtractFinalPlan(text)
		if err == nil {
			t.Fatal("should return error for empty tasks")
		}
		if plan != nil {
			t.Error("plan should be nil on error")
		}
	})

	t.Run("plan embedded in conversational text", func(t *testing.T) {
		text := `Great, here's the plan:

<final_plan>
{
  "project_name": "test",
  "description": "A test project",
  "tech_stack": ["Go"],
  "tasks": [
    {
      "title": "Init project",
      "description": "Set up Go module",
      "acceptance_criteria": ["go.mod exists"],
      "estimated_complexity": "small"
    }
  ]
}
</final_plan>

Let me know if you'd like any changes!`

		plan, err := ExtractFinalPlan(text)
		if err != nil {
			t.Fatalf("ExtractFinalPlan() error: %v", err)
		}
		if plan == nil {
			t.Fatal("plan should not be nil")
		}
		if plan.ProjectName != "test" {
			t.Errorf("ProjectName = %q, want %q", plan.ProjectName, "test")
		}
		if len(plan.Tasks) != 1 {
			t.Errorf("Tasks length = %d, want 1", len(plan.Tasks))
		}
	})
}

func TestExtractPlanUpdate(t *testing.T) {
	t.Run("valid update with mixed actions", func(t *testing.T) {
		text := `<plan_update>
{
  "summary": "Added caching, removed GraphQL",
  "tasks": [
    {"id": "task-001", "action": "keep"},
    {"id": "task-002", "action": "modify", "title": "Updated title", "description": "New desc", "acceptance_criteria": ["new criterion"], "estimated_complexity": "medium"},
    {"action": "add", "title": "New task", "description": "Do something new", "acceptance_criteria": ["it works"], "estimated_complexity": "medium"},
    {"id": "task-003", "action": "remove", "reason": "No longer needed"}
  ]
}
</plan_update>`

		update, err := ExtractPlanUpdate(text)
		if err != nil {
			t.Fatalf("ExtractPlanUpdate() error: %v", err)
		}
		if update == nil {
			t.Fatal("update should not be nil")
		}
		if update.Summary != "Added caching, removed GraphQL" {
			t.Errorf("Summary = %q, want %q", update.Summary, "Added caching, removed GraphQL")
		}
		if len(update.Tasks) != 4 {
			t.Fatalf("Tasks length = %d, want 4", len(update.Tasks))
		}

		// Check keep
		if update.Tasks[0].Action != "keep" {
			t.Errorf("Tasks[0].Action = %q, want %q", update.Tasks[0].Action, "keep")
		}
		if update.Tasks[0].ID != "task-001" {
			t.Errorf("Tasks[0].ID = %q, want %q", update.Tasks[0].ID, "task-001")
		}

		// Check modify
		if update.Tasks[1].Action != "modify" {
			t.Errorf("Tasks[1].Action = %q, want %q", update.Tasks[1].Action, "modify")
		}
		if update.Tasks[1].Title != "Updated title" {
			t.Errorf("Tasks[1].Title = %q, want %q", update.Tasks[1].Title, "Updated title")
		}

		// Check add
		if update.Tasks[2].Action != "add" {
			t.Errorf("Tasks[2].Action = %q, want %q", update.Tasks[2].Action, "add")
		}
		if update.Tasks[2].ID != "" {
			t.Errorf("Tasks[2].ID should be empty for add, got %q", update.Tasks[2].ID)
		}
		if update.Tasks[2].Title != "New task" {
			t.Errorf("Tasks[2].Title = %q, want %q", update.Tasks[2].Title, "New task")
		}

		// Check remove
		if update.Tasks[3].Action != "remove" {
			t.Errorf("Tasks[3].Action = %q, want %q", update.Tasks[3].Action, "remove")
		}
		if update.Tasks[3].Reason != "No longer needed" {
			t.Errorf("Tasks[3].Reason = %q, want %q", update.Tasks[3].Reason, "No longer needed")
		}
	})

	t.Run("no tags — returns nil nil", func(t *testing.T) {
		text := "No plan update here."
		update, err := ExtractPlanUpdate(text)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if update != nil {
			t.Error("update should be nil when no tags present")
		}
	})

	t.Run("tags with invalid JSON", func(t *testing.T) {
		text := `<plan_update>{broken json</plan_update>`
		update, err := ExtractPlanUpdate(text)
		if err == nil {
			t.Fatal("should return error for invalid JSON")
		}
		if update != nil {
			t.Error("update should be nil on error")
		}
	})

	t.Run("task missing action field", func(t *testing.T) {
		text := `<plan_update>
{
  "summary": "test",
  "tasks": [
    {"id": "task-001"}
  ]
}
</plan_update>`
		update, err := ExtractPlanUpdate(text)
		if err == nil {
			t.Fatal("should return error for missing action field")
		}
		if update != nil {
			t.Error("update should be nil on error")
		}
	})

	t.Run("update with dependencies", func(t *testing.T) {
		text := `<plan_update>
{
  "summary": "Added dependency",
  "tasks": [
    {"action": "add", "title": "New", "description": "d", "acceptance_criteria": ["a"], "depends_on": ["task-001", "task-002"], "estimated_complexity": "large"}
  ]
}
</plan_update>`
		update, err := ExtractPlanUpdate(text)
		if err != nil {
			t.Fatalf("ExtractPlanUpdate() error: %v", err)
		}
		if len(update.Tasks[0].DependsOn) != 2 {
			t.Errorf("DependsOn length = %d, want 2", len(update.Tasks[0].DependsOn))
		}
	})
}

func TestParseResponse(t *testing.T) {
	t.Run("valid JSON with result field", func(t *testing.T) {
		raw := []byte(`{"result": "Hello, world!", "session_id": "abc-123"}`)
		resp, err := parseResponse(raw)
		if err != nil {
			t.Fatalf("parseResponse() error: %v", err)
		}
		if resp.Text != "Hello, world!" {
			t.Errorf("Text = %q, want %q", resp.Text, "Hello, world!")
		}
		if resp.SessionID != "abc-123" {
			t.Errorf("SessionID = %q, want %q", resp.SessionID, "abc-123")
		}
		if resp.RawJSON != string(raw) {
			t.Errorf("RawJSON mismatch")
		}
	})

	t.Run("valid JSON with text field", func(t *testing.T) {
		raw := []byte(`{"text": "response text"}`)
		resp, err := parseResponse(raw)
		if err != nil {
			t.Fatalf("parseResponse() error: %v", err)
		}
		if resp.Text != "response text" {
			t.Errorf("Text = %q, want %q", resp.Text, "response text")
		}
	})

	t.Run("valid JSON without known text fields", func(t *testing.T) {
		raw := []byte(`{"data": "something"}`)
		resp, err := parseResponse(raw)
		if err != nil {
			t.Fatalf("parseResponse() error: %v", err)
		}
		// Fallback: entire JSON as text
		if resp.Text != string(raw) {
			t.Errorf("Text = %q, want %q", resp.Text, string(raw))
		}
	})

	t.Run("valid JSON with session_id", func(t *testing.T) {
		raw := []byte(`{"result": "hi", "session_id": "sess-456"}`)
		resp, err := parseResponse(raw)
		if err != nil {
			t.Fatalf("parseResponse() error: %v", err)
		}
		if resp.SessionID != "sess-456" {
			t.Errorf("SessionID = %q, want %q", resp.SessionID, "sess-456")
		}
	})

	t.Run("non-JSON output — plain text fallback", func(t *testing.T) {
		raw := []byte("This is just plain text output from claude")
		resp, err := parseResponse(raw)
		if err != nil {
			t.Fatalf("parseResponse() error: %v", err)
		}
		if resp.Text != string(raw) {
			t.Errorf("Text = %q, want %q", resp.Text, string(raw))
		}
		if resp.RawJSON != string(raw) {
			t.Errorf("RawJSON = %q, want %q", resp.RawJSON, string(raw))
		}
	})

	t.Run("empty input", func(t *testing.T) {
		resp, err := parseResponse([]byte{})
		if err != nil {
			t.Fatalf("parseResponse() error: %v", err)
		}
		if resp.Text != "" {
			t.Errorf("Text = %q, want empty", resp.Text)
		}
	})

	t.Run("result field prefers over text field", func(t *testing.T) {
		raw := []byte(`{"result": "from result", "text": "from text"}`)
		resp, err := parseResponse(raw)
		if err != nil {
			t.Fatalf("parseResponse() error: %v", err)
		}
		if resp.Text != "from result" {
			t.Errorf("Text = %q, want %q (result should take priority)", resp.Text, "from result")
		}
	})
}
