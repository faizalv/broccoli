package queue

import "go/types"

type Queue struct {
	ownNodes   []*types.Named
	finalNodes []*types.Named
}

func New() Queue {
	return Queue{
		ownNodes:   make([]*types.Named, 56),
		finalNodes: make([]*types.Named, 56),
	}
}
