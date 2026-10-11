package chain

// HopResult is one visited node in the chain, in visit order.
type HopResult struct {
	ID      string `json:"id" validate:"required"`
	Message string `json:"message" validate:"required"`
}

// ChainResponse is the success envelope returned by every node.
// Trace holds all visited pong nodes in order; ReceiverID is the last one.
type ChainResponse struct {
	Message    string      `json:"message"`
	CallerID   string      `json:"caller_id"`
	ReceiverID string      `json:"receiver_id"`
	Trace      []HopResult `json:"trace"`
}

// ChainErrorBody is the failure envelope returned when a node injects a
// failure or a downstream call fails. Trace holds the nodes visited so far.
type ChainErrorBody struct {
	Reason   string      `json:"reason"`
	FailedAt string      `json:"failed_at"`
	Trace    []HopResult `json:"trace"`
}
