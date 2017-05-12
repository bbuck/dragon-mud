package talon

import (
	bolt "github.com/johnnadratowski/golang-neo4j-bolt-driver/structures/graph"
)

// Path represents a graph path, like node a -> rel 1 -> node b -> rel 2 ->
// node c or what-have-you.
type Path []interface{}

func wrapBoltPath(bp bolt.Path) (Path, error) {
	p := make(Path, len(bp.Sequence)+1)

	// alternate between nodes and relationships
	isNode := true
	for i, idx := range bp.Sequence {
		// idx is 1-indexed, so we must subtract 1 from in to get the array
		// position
		idx--
		if isNode {
			n, err := wrapBoltNode(bp.Nodes[idx])
			if err != nil {
				return nil, err
			}

			p[i] = n
		} else {
			rel, err := wrapBoltUnboundRelationship(bp.Relationships[idx])
			if err != nil {
				return nil, err
			}

			p[i] = rel
		}
		isNode = !isNode
	}
	n, err := wrapBoltNode(bp.Nodes[len(bp.Nodes)-1])
	if err != nil {
		return nil, err
	}

	p[len(bp.Sequence)] = n

	return p, nil
}
