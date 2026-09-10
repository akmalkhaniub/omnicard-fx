package tests

import (
	"testing"
	"time"

	"github.com/akmalkhaniub/omnicard-fx/internal/domain/cards"
	"github.com/akmalkhaniub/omnicard-fx/internal/domain/fx"
	"github.com/akmalkhaniub/omnicard-fx/internal/jit"
	"github.com/akmalkhaniub/omnicard-fx/internal/repository"
	"github.com/google/uuid"
)

func TestJIT_Sub10msLatency(t *testing.T) {
	store := repository.NewStore()
	fxEngine := fx.NewEngine()
	authorizer := jit.NewAuthorizer(store, fxEngine)

	walletID := uuid.New()
	wallet := &repository.Wallet{
		ID:        walletID,
		Currency:  "USD",
		Balance:   1000000000, // $10,000,000.00
		UpdatedAt: time.Now().UTC(),
	}
	store.SaveWallet(wallet)

	card := &cards.Card{
		ID:             uuid.New(),
		WalletID:       walletID,
		CardholderName: "High Frequency Trader",
		Currency:       "USD",
		Status:         cards.StatusActive,
		SpendingLimits: cards.SpendingLimits{
			DailyCents:          100000000,
			PerTransactionCents: 1000000,
		},
		CreatedAt: time.Now().UTC(),
	}
	store.SaveCard(card)

	const iterations = 1000
	start := time.Now()

	for i := 0; i < iterations; i++ {
		res, err := authorizer.Authorize(card.ID, 100, "USD", "Quick Store", "5411")
		if err != nil || res.Decision != "APPROVED" {
			t.Fatalf("auth failed on iteration %d: %v", i, err)
		}
	}

	totalDuration := time.Since(start)
	avgLatency := totalDuration / time.Duration(iterations)

	t.Logf("Processed %d authorizations in %v (Average latency: %v per auth)", iterations, totalDuration, avgLatency)

	// Strict SLA: average authorization must be under 5 milliseconds (well below the 150ms card network timeout)
	if avgLatency > 5*time.Millisecond {
		t.Errorf("Latency SLA breached: average %v exceeded 5ms", avgLatency)
	}
}

func BenchmarkJITAuthorization(b *testing.B) {
	store := repository.NewStore()
	fxEngine := fx.NewEngine()
	authorizer := jit.NewAuthorizer(store, fxEngine)

	walletID := uuid.New()
	wallet := &repository.Wallet{
		ID:        walletID,
		Currency:  "USD",
		Balance:   1000000000,
		UpdatedAt: time.Now().UTC(),
	}
	store.SaveWallet(wallet)

	card := &cards.Card{
		ID:             uuid.New(),
		WalletID:       walletID,
		CardholderName: "Benchmarker",
		Currency:       "USD",
		Status:         cards.StatusActive,
		SpendingLimits: cards.SpendingLimits{
			DailyCents:          1000000000,
			PerTransactionCents: 10000000,
		},
		CreatedAt: time.Now().UTC(),
	}
	store.SaveCard(card)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = authorizer.Authorize(card.ID, 10, "USD", "Speed Merchant", "5411")
	}
}
