package ports

import "github.com/Im-Manav/ome/pkg/models"

type Matcher interface {
	// Matcher runs price-time priority matching.
	// Pure function — no DB, no Kafka, no Redis. Just domain logic.
	// This is what makes the engine testable and fast.
	Match(order *models.Order) []models.Order
}
