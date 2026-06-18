package models

import "time"

// Prediction is the structured output from the AI predictor.
// Published to Kafka and broadcast to the dashboard via WebSocket.
type Prediction struct {
	Symbol      string    `json:"symbol"`
	Direction   string    `json:"direction"`  // "up" | "down" | "sideways"
	Confidence  int       `json:"confidence"` // 0-100
	Reasoning   string    `json:"reasoning"`  // one-sentence explanation
	GeneratedAt time.Time `json:"generated_at"`
}

// predictionLLMResponse is the raw JSON shape we ask the LLM to return.
// Kept separate from Prediction so a malformed LLM response doesn't
// corrupt the domain model — we validate and convert explicitly.
type predictionLLMResponse struct {
	Direction  string `json:"direction"`
	Confidence int    `json:"confidence"`
	Reasoning  string `json:"reasoning"`
}
