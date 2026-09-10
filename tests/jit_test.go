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

func setupTestEnvironment() (*jit.Authorizer, *repository.Store, *cards.Card, *repository.Wallet) {
	store := repository.NewStore()
	fxEngine := fx.NewEngine()
	authorizer := jit.NewAuthorizer(store, fxEngine)

	walletID := uuid.New()
	wallet := &repository.Wallet{
		ID:        walletID,
		Currency:  "USD",
		Balance:   100000, // $1,000.00 (100,000 cents)
		UpdatedAt: time.Now().UTC(),
	}
	store.SaveWallet(wallet)

	card := &cards.Card{
		ID:             uuid.New(),
		WalletID:       walletID,
		CardholderName: "Alex Mercer",
		Currency:       "USD",
		Status:         cards.StatusActive,
		SpendingLimits: cards.SpendingLimits{
			PerTransactionCents: 25000, // $250.00 max per tx
			DailyCents:          50000, // $500.00 max daily
			MonthlyCents:        200000,
		},
		MCCBlacklist: []string{"7995"}, // Gambling blocked
		CreatedAt:    time.Now().UTC(),
	}
	store.SaveCard(card)

	return authorizer, store, card, wallet
}

func TestJIT_ApprovalSuccess(t *testing.T) {
	authorizer, store, card, wallet := setupTestEnvironment()

	// Purchase: $50.00 at Grocery store (MCC 5411)
	res, err := authorizer.Authorize(card.ID, 5000, "USD", "Whole Foods Market", "5411")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Decision != "APPROVED" || res.ISOResponseCode != "00" {
		t.Errorf("expected APPROVED (00), got %s (%s)", res.Decision, res.ISOResponseCode)
	}

	// Verify wallet deducted
	updatedWallet, _ := store.GetWallet(wallet.ID)
	if updatedWallet.Balance != 95000 { // 100,000 - 5,000 = 95,000 cents
		t.Errorf("expected balance 95000, got %d", updatedWallet.Balance)
	}

	// Verify card daily spend incremented
	updatedCard, _ := store.GetCard(card.ID)
	if updatedCard.DailySpend != 5000 {
		t.Errorf("expected daily spend 5000, got %d", updatedCard.DailySpend)
	}
}

func TestJIT_MCCBlacklistBlocked(t *testing.T) {
	authorizer, _, card, _ := setupTestEnvironment()

	// Gambling purchase: $20.00 (MCC 7995)
	res, err := authorizer.Authorize(card.ID, 2000, "USD", "Vegas Casino Online", "7995")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Decision != "DECLINED" || res.ISOResponseCode != "57" {
		t.Errorf("expected DECLINED (57) for blacklisted MCC, got %s (%s)", res.Decision, res.ISOResponseCode)
	}
}

func TestJIT_ExceedsPerTransactionLimit(t *testing.T) {
	authorizer, _, card, _ := setupTestEnvironment()

	// Purchase of $300 exceeds $250 per-tx limit
	res, err := authorizer.Authorize(card.ID, 30000, "USD", "Electronics Store", "5732")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Decision != "DECLINED" || res.ISOResponseCode != "61" {
		t.Errorf("expected DECLINED (61) for exceeding tx limit, got %s (%s)", res.Decision, res.ISOResponseCode)
	}
}

func TestJIT_CardFrozenDecline(t *testing.T) {
	authorizer, store, card, _ := setupTestEnvironment()

	card.Status = cards.StatusFrozen
	store.UpdateCard(card)

	res, err := authorizer.Authorize(card.ID, 1500, "USD", "Coffee Shop", "5814")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Decision != "DECLINED" || res.ISOResponseCode != "57" {
		t.Errorf("expected DECLINED (57) for frozen card, got %s (%s)", res.Decision, res.ISOResponseCode)
	}
}

func TestJIT_CrossCurrencyFXConversion(t *testing.T) {
	authorizer, store, card, wallet := setupTestEnvironment()

	// Purchase in Amsterdam: 100.00 EUR (Card is in USD)
	// EUR/USD rate is ~1.0870 + 50bps spread = ~1.0924
	res, err := authorizer.Authorize(card.ID, 10000, "EUR", "Amsterdam Cafe", "5812")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Decision != "APPROVED" || res.ISOResponseCode != "00" {
		t.Fatalf("expected APPROVED for cross-currency, got %s: %s", res.Decision, res.DeclineReason)
	}

	if res.SettledCurrency != "USD" {
		t.Errorf("expected settled currency USD, got %s", res.SettledCurrency)
	}
	if res.SettledAmountCents <= 10000 {
		t.Errorf("expected settled amount in USD to reflect EUR conversion markup, got %d", res.SettledAmountCents)
	}

	// Verify wallet was deducted in USD
	updatedWallet, _ := store.GetWallet(wallet.ID)
	expectedBal := 100000 - res.SettledAmountCents
	if updatedWallet.Balance != expectedBal {
		t.Errorf("expected wallet balance %d, got %d", expectedBal, updatedWallet.Balance)
	}
}
