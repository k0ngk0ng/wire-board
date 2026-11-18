package engine

import (
	"context"
	"errors"
	"fmt"
)

type RuntimeIdentity struct {
	BuildID      string `json:"buildID"`
	ResourceID   string `json:"resourceID"`
	Profile      string `json:"profile"`
	Purpose      string `json:"purpose"`
	SourceCommit string `json:"sourceCommit"`
}

func (r *Runtime) Identity() RuntimeIdentity {
	info := RuntimeIdentity{BuildID: r.buildID, ResourceID: r.resourceID}
	if r.provenance != nil {
		info.Profile, info.Purpose, info.SourceCommit = r.provenance.Profile, r.provenance.Purpose, r.provenance.SourceCommit
	}
	return info
}

type runtimeKey struct{ build, resources string }

var ErrRuntimeUnavailable = errors.New("PAL save requires unavailable native/resource version")

type RuntimeCatalog struct {
	current  *Runtime
	versions map[runtimeKey]*Runtime
}

func NewRuntimeCatalog(current *Runtime, retained ...*Runtime) (*RuntimeCatalog, error) {
	if current == nil {
		return nil, errors.New("PAL current native version is missing")
	}
	c := &RuntimeCatalog{current: current, versions: map[runtimeKey]*Runtime{}}
	for _, r := range append([]*Runtime{current}, retained...) {
		if r == nil || r.provenance == nil || r.provenance.Purpose != "campaign" {
			return nil, errors.New("PAL runtime catalog requires classified campaign cores")
		}
		if r.provenance.Profile != current.provenance.Profile {
			return nil, errors.New("PAL original and development profiles cannot share a runtime catalog")
		}
		key := runtimeKey{r.buildID, r.resourceID}
		if c.versions[key] != nil {
			return nil, errors.New("duplicate PAL native/resource version")
		}
		c.versions[key] = r
	}
	return c, nil
}

func (c *RuntimeCatalog) CurrentIdentity() RuntimeIdentity { return c.current.Identity() }

func (c *RuntimeCatalog) StartCampaignContinuationForPlayers(ctx context.Context, seed uint32, players int) (*Continuation, error) {
	return c.current.StartCampaignContinuationForPlayers(ctx, seed, players)
}

func (c *RuntimeCatalog) Continue(encoded []byte) (*Continuation, error) {
	s, err := decodeSnapshot(encoded)
	if err != nil {
		return nil, err
	}
	r := c.versions[runtimeKey{s.BuildID, s.ResourceID}]
	if r == nil {
		return nil, fmt.Errorf("%w %s/%s", ErrRuntimeUnavailable, s.BuildID, s.ResourceID)
	}
	if err := r.validateMetadata(s); err != nil {
		return nil, err
	}
	return newContinuation(r, s), nil
}
