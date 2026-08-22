// Package report turns persisted campaign snapshots into stable operational
// reports. It contains no database code so recovery, HTTP and future exports
// can use exactly the same aggregation and timeline rules.
package report

import (
	"sort"

	"task143-batchreactor/internal/model"
)

// TimelineWindow is one planned or actual occupancy window on a reactor. The
// window is intentionally small and serializable because it is shared by the
// campaign report and the frontend's Gantt-style timeline.
type TimelineWindow struct {
	ReactorID string            `json:"reactor_id"`
	BatchID   string            `json:"batch_id"`
	RecipeID  string            `json:"recipe_id"`
	Status    model.BatchStatus `json:"status"`
	Start     int64             `json:"start"`
	End       int64             `json:"end"`
	Planned   bool              `json:"planned"`
}

// Gap is an idle gap or overlap between successive reactor windows. Positive
// Seconds means idle time; negative Seconds means an invalid overlap that the
// caller should surface before campaign execution.
type Gap struct {
	ReactorID string `json:"reactor_id"`
	BeforeID  string `json:"before_id"`
	AfterID   string `json:"after_id"`
	Seconds   int64  `json:"seconds"`
}

// Windows creates one stable timeline window per batch. For started batches,
// the actual start is preferred. Terminal batches use their recorded end;
// non-terminal batches use the expected duration supplied by the caller.
func Windows(batches []model.Batch, expected map[string]int64, now int64) []TimelineWindow {
	out := make([]TimelineWindow, 0, len(batches))
	for _, b := range batches {
		start := b.PlannedStart
		planned := true
		if b.StartedAt > 0 {
			start, planned = b.StartedAt, false
		}
		if start == 0 {
			start = now
		}
		end := b.EndedAt
		if end == 0 {
			d := expected[b.ID]
			if d < 1 {
				d = 1
			}
			end = start + d
		}
		if end < start {
			end = start
		}
		out = append(out, TimelineWindow{ReactorID: b.ReactorID, BatchID: b.ID, RecipeID: b.RecipeID, Status: b.Status, Start: start, End: end, Planned: planned})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ReactorID != out[j].ReactorID {
			return out[i].ReactorID < out[j].ReactorID
		}
		if out[i].Start != out[j].Start {
			return out[i].Start < out[j].Start
		}
		return out[i].BatchID < out[j].BatchID
	})
	return out
}

// Gaps returns all successive gaps for each reactor. It does not hide
// overlaps: a negative duration is critical planning evidence, while a zero
// duration indicates a direct handoff from one batch to the next.
func Gaps(windows []TimelineWindow) []Gap {
	if len(windows) < 2 {
		return []Gap{}
	}
	out := make([]Gap, 0, len(windows)-1)
	var previous *TimelineWindow
	for i := range windows {
		cur := &windows[i]
		if previous == nil || previous.ReactorID != cur.ReactorID {
			previous = cur
			continue
		}
		out = append(out, Gap{ReactorID: cur.ReactorID, BeforeID: previous.BatchID, AfterID: cur.BatchID, Seconds: cur.Start - previous.End})
		if cur.End > previous.End {
			previous = cur
		}
	}
	return out
}

// Span returns the earliest start and latest end in a timeline. Empty input
// has a zero span; callers can therefore distinguish it from a real epoch.
func Span(windows []TimelineWindow) (int64, int64) {
	if len(windows) == 0 {
		return 0, 0
	}
	start, end := windows[0].Start, windows[0].End
	for _, w := range windows[1:] {
		if w.Start < start {
			start = w.Start
		}
		if w.End > end {
			end = w.End
		}
	}
	return start, end
}

// ByReactor groups an already-stable timeline without sharing backing arrays,
// which prevents a report consumer from accidentally mutating another
// reactor's visible schedule.
func ByReactor(windows []TimelineWindow) map[string][]TimelineWindow {
	out := make(map[string][]TimelineWindow)
	for _, w := range windows {
		out[w.ReactorID] = append(out[w.ReactorID], w)
	}
	return out
}

// Overlaps returns only conflicting consecutive windows. It is deliberately a
// separate helper because reports normally preserve all gaps, while safety
// validation needs a compact list of schedule violations.
func Overlaps(windows []TimelineWindow) []Gap {
	gaps := Gaps(windows)
	out := make([]Gap, 0)
	for _, g := range gaps {
		if g.Seconds < 0 {
			out = append(out, g)
		}
	}
	return out
}
