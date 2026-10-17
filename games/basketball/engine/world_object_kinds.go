package engine

// World kinds are derived at the consuming phase. Court4 changes auxiliary
// kinds during motion; the immutable opening catalog is only a fallback for
// existing fixed-court states, never a second current kind for those objects.
func worldObjectKinds(s WorldActionState, catalog [13]uint8) [13]uint8 {
	for i, a := range s.Auxiliary {
		if a.Kind != 0 {
			catalog[i+4] = a.Kind
		}
	}
	return catalog
}

func worldAuxiliaryKind(s WorldActionState, court uint8, index int) uint8 {
	if k := s.Auxiliary[index].Kind; k != 0 {
		return k
	}
	kinds := [4][2]uint8{{10, 11}, {12, 11}, {12, 13}, {13, 14}}
	return kinds[court&3][index]
}
