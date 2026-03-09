package queue

import "go/types"

// Queue stores named structs scheduled for code generation.
type Queue struct {
	nodes []*types.Named
	seen  map[*types.Named]bool
}

func New() *Queue {
	return &Queue{
		nodes: make([]*types.Named, 0, 64),
		seen:  make(map[*types.Named]bool),
	}
}

// Enqueue adds a named type if it is unseen. Embedded types are tracked but not emitted.
func (q *Queue) Enqueue(n *types.Named, embedded bool) {
	if n == nil || q.seen[n] {
		return
	}
	q.seen[n] = true
	if !embedded {
		q.nodes = append(q.nodes, n)
	}
}

func (q *Queue) Nodes() []*types.Named {
	return q.nodes
}
