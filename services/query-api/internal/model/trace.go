package model

import (
	"sort"
	"time"
)

// Trace represents a complete distributed trace comprised of correlated spans.
type Trace struct {
	TraceID       string    `json:"traceId"`
	Spans         []*Span   `json:"spans"`
	Root          *SpanNode `json:"root,omitempty"`
	CriticalPath  []*Span   `json:"criticalPath,omitempty"`
	TotalDuration uint64    `json:"totalDurationNs"`
}

// TraceSummary provides a lightweight view for searchTraces results.
type TraceSummary struct {
	TraceID       string    `json:"traceId"`
	RootService   string    `json:"rootService"`
	RootOperation string    `json:"rootOperation"`
	StartTime     time.Time `json:"startTime"`
	DurationNs    uint64    `json:"durationNs"`
	SpanCount     int       `json:"spanCount"`
	HasError      bool      `json:"hasError"`
}

// BuildTree constructs a hierarchical tree of SpanNodes from flat spans.
// It links spans using ParentSpanID rather than timestamps to handle inter-host clock skew.
// Spans whose parents are missing or orphaned are attached under a synthetic or primary root.
func (t *Trace) BuildTree() *SpanNode {
	if len(t.Spans) == 0 {
		return nil
	}

	nodes := make(map[string]*SpanNode, len(t.Spans))
	for _, span := range t.Spans {
		nodes[span.SpanID] = &SpanNode{
			Span:     span,
			Children: make([]*SpanNode, 0),
			Depth:    0,
		}
	}

	var rootNodes []*SpanNode

	for _, span := range t.Spans {
		node := nodes[span.SpanID]
		if span.ParentSpanID == "" {
			rootNodes = append(rootNodes, node)
		} else if parentNode, exists := nodes[span.ParentSpanID]; exists {
			parentNode.Children = append(parentNode.Children, node)
		} else {
			// Orphaned span whose parent wasn't captured in the window; treat as top-level candidate
			rootNodes = append(rootNodes, node)
		}
	}

	// Sort children by start time for consistent chronological rendering
	var assignDepthAndSort func(node *SpanNode, depth int)
	assignDepthAndSort = func(node *SpanNode, depth int) {
		node.Depth = depth
		sort.Slice(node.Children, func(i, j int) bool {
			return node.Children[i].Span.StartTime.Before(node.Children[j].Span.StartTime)
		})
		for _, child := range node.Children {
			assignDepthAndSort(child, depth+1)
		}
	}

	var root *SpanNode
	if len(rootNodes) == 1 {
		root = rootNodes[0]
		assignDepthAndSort(root, 0)
	} else if len(rootNodes) > 1 {
		// Synthesize a parent root container if multiple disconnected root spans exist
		minStart := t.Spans[0].StartTime
		var maxEnd time.Time
		for _, s := range t.Spans {
			if s.StartTime.Before(minStart) {
				minStart = s.StartTime
			}
			end := s.StartTime.Add(time.Duration(s.DurationNs))
			if end.After(maxEnd) {
				maxEnd = end
			}
		}
		syntheticDuration := uint64(maxEnd.Sub(minStart))

		syntheticSpan := &Span{
			TraceID:     t.TraceID,
			SpanID:      "root-synthetic",
			ServiceName: rootNodes[0].Span.ServiceName,
			Name:        "trace-root",
			Kind:        SpanKindServer,
			StartTime:   minStart,
			DurationNs:  syntheticDuration,
			StatusCode:  StatusCodeOK,
		}
		root = &SpanNode{
			Span:     syntheticSpan,
			Children: rootNodes,
			Depth:    0,
		}
		assignDepthAndSort(root, 0)
	}

	t.Root = root
	t.TotalDuration = t.CalculateTotalDuration()
	t.CriticalPath = t.GetCriticalPath()
	return root
}

// CalculateTotalDuration computes the total wall-clock duration of the trace in nanoseconds.
func (t *Trace) CalculateTotalDuration() uint64 {
	if len(t.Spans) == 0 {
		return 0
	}
	minStart := t.Spans[0].StartTime
	maxEnd := t.Spans[0].StartTime.Add(time.Duration(t.Spans[0].DurationNs))

	for _, s := range t.Spans[1:] {
		if s.StartTime.Before(minStart) {
			minStart = s.StartTime
		}
		end := s.StartTime.Add(time.Duration(s.DurationNs))
		if end.After(maxEnd) {
			maxEnd = end
		}
	}

	diff := maxEnd.Sub(minStart)
	if diff < 0 {
		return 0
	}
	return uint64(diff)
}

// GetCriticalPath finds the sequence of spans that dictated the total trace duration.
// In a distributed execution DAG, this traverses from root along the longest duration path.
func (t *Trace) GetCriticalPath() []*Span {
	if t.Root == nil {
		t.BuildTree()
	}
	if t.Root == nil {
		return nil
	}

	var path []*Span

	var findLongestBranch func(node *SpanNode) []*Span
	findLongestBranch = func(node *SpanNode) []*Span {
		if node == nil || node.Span == nil {
			return nil
		}

		currentList := []*Span{node.Span}
		if len(node.Children) == 0 {
			return currentList
		}

		var bestChildPath []*Span
		var maxChildDuration uint64

		for _, child := range node.Children {
			childPath := findLongestBranch(child)
			var branchDuration uint64
			for _, s := range childPath {
				branchDuration += s.DurationNs
			}
			if branchDuration >= maxChildDuration {
				maxChildDuration = branchDuration
				bestChildPath = childPath
			}
		}

		return append(currentList, bestChildPath...)
	}

	path = findLongestBranch(t.Root)

	// Filter out synthetic root from the critical path output
	var filtered []*Span
	for _, s := range path {
		if s.SpanID != "root-synthetic" {
			filtered = append(filtered, s)
		}
	}
	return filtered
}
