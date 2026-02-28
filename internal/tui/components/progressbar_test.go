package components

import (
	"strings"
	"testing"
)

func TestNewProgressBarModel(t *testing.T) {
	t.Parallel()
	m := NewProgressBarModel(10, 40)

	if m.total != 10 {
		t.Errorf("total = %d, want 10", m.total)
	}
	if m.width != 40 {
		t.Errorf("width = %d, want 40", m.width)
	}
	if m.done != 0 {
		t.Errorf("done = %d, want 0", m.done)
	}
}

func TestProgressBar_ZeroTotal(t *testing.T) {
	t.Parallel()
	m := NewProgressBarModel(0, 40)

	view := m.View()
	if !strings.Contains(view, "0/0") {
		t.Errorf("zero total view should contain '0/0', got %q", view)
	}
	if !strings.Contains(view, "0%") {
		t.Errorf("zero total view should contain '0%%', got %q", view)
	}
}

func TestProgressBar_HalfDone(t *testing.T) {
	t.Parallel()
	m := NewProgressBarModel(10, 40)
	m.SetDone(5)

	view := m.View()
	if !strings.Contains(view, "50%") {
		t.Errorf("half done view should contain '50%%', got %q", view)
	}
	if !strings.Contains(view, "5/10") {
		t.Errorf("half done view should contain '5/10', got %q", view)
	}
}

func TestProgressBar_AllDone(t *testing.T) {
	t.Parallel()
	m := NewProgressBarModel(8, 40)
	m.SetDone(8)

	view := m.View()
	if !strings.Contains(view, "100%") {
		t.Errorf("all done view should contain '100%%', got %q", view)
	}
	if !strings.Contains(view, "8/8") {
		t.Errorf("all done view should contain '8/8', got %q", view)
	}
}

func TestProgressBar_SetDone(t *testing.T) {
	t.Parallel()
	m := NewProgressBarModel(10, 40)
	m.SetDone(3)

	if m.done != 3 {
		t.Errorf("done = %d, want 3", m.done)
	}
}

func TestProgressBar_SetTotal(t *testing.T) {
	t.Parallel()
	m := NewProgressBarModel(10, 40)
	m.SetTotal(20)

	if m.total != 20 {
		t.Errorf("total = %d, want 20", m.total)
	}
}

func TestProgressBar_SetWidth(t *testing.T) {
	t.Parallel()
	m := NewProgressBarModel(10, 40)
	m.SetWidth(60)

	if m.width != 60 {
		t.Errorf("width = %d, want 60", m.width)
	}
}

func TestProgressBar_View_ContainsPercentage(t *testing.T) {
	t.Parallel()
	m := NewProgressBarModel(4, 40)
	m.SetDone(1)

	view := m.View()
	if !strings.Contains(view, "25%") {
		t.Errorf("view should contain '25%%', got %q", view)
	}
}

func TestProgressBar_View_ContainsFraction(t *testing.T) {
	t.Parallel()
	m := NewProgressBarModel(4, 40)
	m.SetDone(1)

	view := m.View()
	if !strings.Contains(view, "1/4") {
		t.Errorf("view should contain '1/4', got %q", view)
	}
}
