package contra

import "errors"

// AdvanceQueued applies the rule/collision part and appends its deferred
// graphics as one transaction. The packet grouping is the original grouping,
// so subsequent enemy requests observe the correct byte-based admission limit.
func (s *JungleScroll) AdvanceQueued(world *OutdoorWorld, terrain *CollisionWindow, queue *GraphicsQueue) (ScrollResult, error) {
	if world == nil || terrain == nil || queue == nil {
		return ScrollResult{}, errors.New("incomplete queued scroll context")
	}
	nextState, nextWorld, nextTerrain := *s, *world, *terrain
	nextQueue := queue.Clone()
	result, err := nextState.Advance(&nextWorld, &nextTerrain)
	if err != nil {
		return ScrollResult{}, err
	}
	for _, write := range result.Writes {
		var packet GraphicsPacket
		switch write.Step {
		case 32:
			if len(write.Values) == 0 || len(write.Values) > 255 {
				return ScrollResult{}, errors.New("invalid scrolling tile column")
			}
			packet = GraphicsPacket{Kind: 2, Length: uint8(len(write.Values)), Segments: []GraphicsSegment{{Address: write.Address, Values: write.Values}}}
		case 8:
			packet = GraphicsPacket{Kind: 1, Length: 1}
			for i, value := range write.Values {
				packet.Segments = append(packet.Segments, GraphicsSegment{Address: write.Address + uint16(i)*8, Values: []uint8{value}})
			}
		default:
			return ScrollResult{}, errors.New("unknown scrolling graphics stride")
		}
		if err = nextQueue.Append(packet); err != nil {
			return ScrollResult{}, err
		}
	}
	*s, *world, *terrain, *queue = nextState, nextWorld, nextTerrain, nextQueue
	return result, nil
}
