package tick
// Triage job payload:
type triagePayload struct {
    EpisodeID string `json:"episode_id"`
}
// Extract job payload:
type extractPayload struct {
    Batch []extract.EpisodePayload `json:"batch"`
}
// Reconcile payload:
type reconcilePayload struct {
    Candidate extract.Candidate `json:"candidate"`
}
// Verify payload:
type verifyPayload struct {
    ClaimID string `json:"claim_id"`
}
// Publish payload:
type publishPayload struct {
    ClaimID string `json:"claim_id"`
}
