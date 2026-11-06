package pcmassets

import "sort"

// Descriptors returns detached immutable value references, sorted by logical
// ID. Encoded payloads remain solely in the catalog; no bytes are copied here.
func (c *Catalog) Descriptors() []Descriptor {
	if c == nil {
		return nil
	}
	result := make([]Descriptor, 0, len(c.descriptors))
	for _, descriptor := range c.descriptors {
		result = append(result, descriptor)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}
