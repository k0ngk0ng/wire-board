package contra

// A visibly swimming teammate demonstrates the first jungle's lower route.
// A trailing player can otherwise lock the shared camera by repeatedly jumping
// back to the high bank. This uses visible positions/artwork and remembered
// public stage information, never a terrain window or the peer's rule state.
func (b *Bot) followSwimmingPeer(o BotObservation) bool {
	if o.Seats != 2 || o.Actor < 0 || o.Actor > 1 || o.Vertical || o.Scene != 0 || o.Stage != 0 || !(o.StageKnown || b.StageKnown) {
		return false
	}
	p, peer := o.Players[o.Actor], o.Players[o.Actor^1]
	if !p.Visible || !peer.Visible || !p.Alive || !peer.Alive || p.Y >= 180 || peer.Y < 190 || peer.Y-p.Y < 64 || peer.X-p.X < 64 {
		return false
	}
	if p.Sprite >= 10 && p.Sprite <= 12 || peer.Sprite >= 10 && peer.Sprite <= 12 {
		return false
	}
	water := false
	for _, candidate := range o.PlayerCandidates {
		q := candidate.Player
		if candidate.Exact && candidate.Mask&(1<<uint(o.Actor^1)) != 0 && q.Visible && q.Sprite >= 24 && q.Sprite <= 29 && absBot(q.X-peer.X) <= 8 && absBot(q.Y-peer.Y) <= 8 {
			water = true
			break
		}
	}
	if !water {
		return false
	}
	if target, _, _, found := b.planJump(o, false); found && target > p.X+12 {
		return false
	}
	return true
}
