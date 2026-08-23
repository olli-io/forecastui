package render

import (
	"strings"
	"testing"
)

func TestCursorFramesTheColumn(t *testing.T) {
	cols := demoColumns(12)
	o := Opts{Count: 8, Cursor: 3}
	lines := Cursor(Chart(cols, NewScale(cols), o), cols, o)

	// Two rows are added: the cap, and the foot that carries the arrow.
	chart := Chart(cols, NewScale(cols), o)
	if len(lines) != len(chart)+2 {
		t.Fatalf("framing added %d rows, want 2", len(lines)-len(chart))
	}

	left := AxisW + 3*Step - 1
	head := []rune(lines[0].Plain())
	if head[left] != '┌' || head[left+Step-1] != '┐' {
		t.Errorf("cap is %q", strings.TrimRight(string(head), " "))
	}
	for i, l := range lines[1 : len(lines)-1] {
		row := []rune(l.Plain())
		if got := row[left]; got != '│' && got != '┼' && got != '├' {
			t.Errorf("row %d: left wall is %q", i, got)
		}
		if got := row[left+Step-1]; got != '│' && got != '┼' {
			t.Errorf("row %d: right wall is %q", i, got)
		}
	}

	foot := []rune(lines[len(lines)-1].Plain())
	if foot[left] != '└' || foot[left+1] != '▲' || foot[left+Step-1] != '┘' {
		t.Errorf("foot is %q", strings.TrimRight(string(foot), " "))
	}
}

func TestCursorOutsideTheWindowDrawsNothing(t *testing.T) {
	cols := demoColumns(12)
	for _, cursor := range []int{-1, 9} {
		o := Opts{Count: 8, Cursor: cursor}
		chart := Chart(cols, NewScale(cols), o)
		if got := Cursor(chart, cols, o); len(got) != len(chart) {
			t.Errorf("cursor %d: framed anyway", cursor)
		}
	}
}

// Only the arrow carries the accent; the frame around it stays darker.
func TestCursorArrowIsLit(t *testing.T) {
	cols := demoColumns(12)
	o := Opts{Count: 8, Cursor: 3}
	lines := Cursor(Chart(cols, NewScale(cols), o), cols, o)

	foot := lines[len(lines)-1]
	var lit, frame string
	for _, s := range foot {
		switch s.Colour {
		case Yellow:
			lit += s.Text
		case Dim:
			frame += s.Text
		}
	}
	if lit != "▲" {
		t.Errorf("lit %q, want the cursor arrow", lit)
	}
	if !strings.Contains(frame, "└") || !strings.Contains(frame, "┘") {
		t.Errorf("the frame around it should stay dim, got %q", frame)
	}
}
