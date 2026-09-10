package jit

import (
	"fmt"
	"time"

	"github.com/akmalkhaniub/omnicard-fx/internal/domain/cards"
	"github.com/akmalkhaniub/omnicard-fx/internal/domain/fx"
	"github.com/akmalkhaniub/omnicard-fx/internal/repository"
	"github.com/google/uuid"
)

type DecisionResult struct {
	ID                 uuid.UUID
	Decision           string // APPROVED / DECLINED
	ISOResponseCode    string // 00, 51, 54, 57, 61
	DeclineReason      string
	SettledAmountCents int64
	SettledCurrency    string
	FXRate             float64
	LatencyMs          int64
	CreatedAt          time.Time
}

type Authorizer struct {
	store    *repository.Store
	fxEngine *fx.Engine
}

func NewAuthorizer(store *repository.Store, fxEngine *fx.Engine) *Authorizer {
	return &Authorizer{
		store:    store,
		fxEngine: fxEngine,
	}
}

func (a *Authorizer) Authorize(cardID uuid.UUID, amountCents int64, currency string, merchant string, mcc string) (*DecisionResult, error) {
	start := time.Now()

	card, err := a.store.GetCard(cardID)
	if err != nil {
		return a.decline(cardID, "57", "Card does not exist", amountCents, currency, 0, start), nil
	}

	// 1. Status Check
	if card.Status != cards.StatusActive {
		return a.decline(cardID, "57", fmt.Sprintf("Card is %s", card.Status), amountCents, card.Currency, 0, start), nil
	}

	// 2. MCC Filter Check
	if len(card.MCCBlacklist) > 0 {
		for _, blockedMCC := range card.MCCBlacklist {
			if blockedMCC == mcc {
				return a.decline(cardID, "57", fmt.Sprintf("Merchant Category Code %s is blocked", mcc), amountCents, card.Currency, 0, start), nil
			}
		}
	}
	if len(card.MCCWhitelist) > 0 {
		allowed := false
		for _, allowedMCC := range card.MCCWhitelist {
			if allowedMCC == mcc {
				allowed = true
				break
			}
		}
		if !allowed {
			return a.decline(cardID, "57", fmt.Sprintf("Merchant Category Code %s is not permitted", mcc), amountCents, card.Currency, 0, start), nil
		}
	}

	// 3. Spending Limit & Velocity Checks
	if card.SpendingLimits.PerTransactionCents > 0 && amountCents > card.SpendingLimits.PerTransactionCents {
		return a.decline(cardID, "61", fmt.Sprintf("Amount %d exceeds per-transaction limit %d", amountCents, card.SpendingLimits.PerTransactionCents), amountCents, card.Currency, 0, start), nil
	}

	if card.SpendingLimits.DailyCents > 0 && (card.DailySpend+amountCents) > card.SpendingLimits.DailyCents {
		return a.decline(cardID, "61", fmt.Sprintf("Amount exceeds daily spend limit %d", card.SpendingLimits.DailyCents), amountCents, card.Currency, 0, start), nil
	}

	// 4. Currency Conversion (Dynamic FX)
	settledAmount := amountCents
	settledCurrency := card.Currency
	fxRate := 1.0

	if currency != card.Currency {
		rate, err := a.fxEngine.GetRate(currency, card.Currency, 50) // 50 bps spread
		if err != nil {
			return a.decline(cardID, "57", fmt.Sprintf("Currency conversion error: %v", err), amountCents, card.Currency, 0, start), nil
		}
		fxRate = rate
		settledAmount = int64(float64(amountCents) * rate)
	}

	// 5. Wallet Balance Check
	wallet, err := a.store.GetWallet(card.WalletID)
	if err != nil {
		return a.decline(cardID, "51", "Cardholder funding wallet not found", settledAmount, settledCurrency, fxRate, start), nil
	}

	if wallet.Balance < settledAmount {
		return a.decline(cardID, "51", fmt.Sprintf("Insufficient funds: available %d, required %d", wallet.Balance, settledAmount), settledAmount, settledCurrency, fxRate, start), nil
	}

	// 6. Execute atomic deduction and velocity increment
	if err := a.store.DeductWalletBalance(card.WalletID, settledAmount); err != nil {
		return a.decline(cardID, "51", "Failed to lock wallet balance", settledAmount, settledCurrency, fxRate, start), nil
	}

	card.DailySpend += settledAmount
	card.MonthlySpend += settledAmount
	a.store.UpdateCard(card)

	latency := time.Since(start).Milliseconds()
	authID := uuid.New()

	rec := &repository.AuthorizationRecord{
		ID:                 authID,
		CardID:             card.ID,
		Decision:           "APPROVED",
		ISOResponseCode:    "00",
		OriginalAmount:     amountCents,
		OriginalCurrency:   currency,
		SettledAmountCents: settledAmount,
		SettledCurrency:    settledCurrency,
		FXRate:             fxRate,
		MerchantName:       merchant,
		MCC:                mcc,
		LatencyMs:          latency,
		CreatedAt:          time.Now().UTC(),
	}
	a.store.SaveAuthorization(rec)

	return &DecisionResult{
		ID:                 authID,
		Decision:           "APPROVED",
		ISOResponseCode:    "00",
		SettledAmountCents: settledAmount,
		SettledCurrency:    settledCurrency,
		FXRate:             fxRate,
		LatencyMs:          latency,
		CreatedAt:          rec.CreatedAt,
	}, nil
}

func (a *Authorizer) decline(cardID uuid.UUID, isoCode string, reason string, amount int64, currency string, fxRate float64, start time.Time) *DecisionResult {
	latency := time.Since(start).Milliseconds()
	authID := uuid.New()

	rec := &repository.AuthorizationRecord{
		ID:                 authID,
		CardID:             cardID,
		Decision:           "DECLINED",
		ISOResponseCode:    isoCode,
		DeclineReason:      reason,
		OriginalAmount:     amount,
		OriginalCurrency:   currency,
		SettledAmountCents: amount,
		SettledCurrency:    currency,
		FXRate:             fxRate,
		LatencyMs:          latency,
		CreatedAt:          time.Now().UTC(),
	}
	a.store.SaveAuthorization(rec)

	return &DecisionResult{
		ID:                 authID,
		Decision:           "DECLINED",
		ISOResponseCode:    isoCode,
		DeclineReason:      reason,
		SettledAmountCents: amount,
		SettledCurrency:    currency,
		FXRate:             fxRate,
		LatencyMs:          latency,
		CreatedAt:          rec.CreatedAt,
	}
}
