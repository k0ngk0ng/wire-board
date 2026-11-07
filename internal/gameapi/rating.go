package gameapi

// RatingPolicy selects a platform-owned versioned formula. Modules declare
// outcomes; neither the game payload nor a client supplies point deltas.
type RatingPolicy string

// RatingCoopCampaignV1 gives every eligible participant +20 on a shared victory
// or -10 on shared defeat/abandonment. Existing matches are never opted in later.
const RatingCoopCampaignV1 RatingPolicy = "coop_campaign_v1"
