package fx

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Quote struct {
	ID                   uuid.UUID
	SourceCurrency       string
	TargetCurrency       string
	Rate                 float64
	SpreadBps            int
	OriginalAmountCents  int64
	ConvertedAmountCents int64
	ExpiresAt            time.Time
}

type Engine struct {
	mu     sync.RWMutex
	rates  map[string]float64 // USD base pairs (e.g. USD/EUR, USD/GBP)
	quotes map[uuid.UUID]*Quote
}

func NewEngine() *Engine {
	e := &Engine{
		rates: map[string]float64{
			"USD:EUR": 0.9200, // 1 USD = 0.92 EUR
			"EUR:USD": 1.0870, // 1 EUR = 1.087 USD
			"USD:GBP": 0.7850, // 1 USD = 0.785 GBP
			"GBP:USD": 1.2738, // 1 GBP = 1.2738 USD
			"EUR:GBP": 0.8530,
			"GBP:EUR": 1.1723,
			"USD:USD": 1.0000,
			"EUR:EUR": 1.0000,
			"GBP:GBP": 1.0000,
		},
		quotes: make(map[uuid.UUID]*Quote),
	}
	return e
}

// GetRate retrieves exchange rate with spread applied
func (e *Engine) GetRate(source string, target string, spreadBps int) (float64, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if source == target {
		return 1.0, nil
	}

	key := fmt.Sprintf("%s:%s", source, target)
	baseRate, exists := e.rates[key]
	if !exists {
		return 0, fmt.Errorf("currency pair %s not supported", key)
	}

	// Apply spread (e.g. 50 bps = 0.50% markup)
	spreadMultiplier := 1.0 + (float64(spreadBps) / 10000.0)
	finalRate := baseRate * spreadMultiplier
	return math.Round(finalRate*10000) / 10000, nil
}

// CreateGuaranteedQuote generates a 60-second guaranteed FX conversion quote
func (e *Engine) CreateGuaranteedQuote(source string, target string, amountCents int64, spreadBps int) (*Quote, error) {
	rate, err := e.GetRate(source, target, spreadBps)
	if err != nil {
		return nil, err
	}

	convertedCents := int64(math.Round(float64(amountCents) * rate))

	quote := &Quote{
		ID:                   uuid.New(),
		SourceCurrency:       source,
		TargetCurrency:       target,
		Rate:                 rate,
		SpreadBps:            spreadBps,
		OriginalAmountCents:  amountCents,
		ConvertedAmountCents: convertedCents,
		ExpiresAt:            time.Now().UTC().Add(60 * time.Second),
	}

	e.mu.Lock()
	e.quotes[quote.ID] = quote
	e.mu.Unlock()

	return quote, nil
}
