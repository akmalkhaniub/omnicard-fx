package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/akmalkhaniub/omnicard-fx/internal/compliance"
	"github.com/akmalkhaniub/omnicard-fx/internal/domain/cards"
	"github.com/akmalkhaniub/omnicard-fx/internal/domain/fx"
	gen "github.com/akmalkhaniub/omnicard-fx/internal/generated/api"
	"github.com/akmalkhaniub/omnicard-fx/internal/jit"
	"github.com/akmalkhaniub/omnicard-fx/internal/repository"
	"github.com/google/uuid"
)

type CardAPI struct {
	store      *repository.Store
	fxEngine   *fx.Engine
	authorizer *jit.Authorizer
	compliance *compliance.Service
}

func NewCardAPI(store *repository.Store, fxEngine *fx.Engine) *CardAPI {
	return &CardAPI{
		store:      store,
		fxEngine:   fxEngine,
		authorizer: jit.NewAuthorizer(store, fxEngine),
		compliance: compliance.NewService(store),
	}
}

// IssueCard handles POST /v1/cards
func (a *CardAPI) IssueCard(w http.ResponseWriter, r *http.Request) {
	var req gen.IssueCardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		gen.RespondProblem(w, http.StatusBadRequest, "MALFORMED_JSON", "Invalid JSON payload")
		return
	}

	if req.CardholderName == "" || req.Currency == "" {
		gen.RespondProblem(w, http.StatusBadRequest, "VALIDATION_FAILED", "cardholder_name and currency are required")
		return
	}

	cardType := cards.TypeVirtual
	if req.CardType != nil {
		cardType = cards.CardType(*req.CardType)
	}

	fullPAN, maskedPAN := cards.GeneratePAN()
	now := time.Now().UTC()

	// Seed or fetch funding wallet
	walletID := uuid.New()
	wallet := &repository.Wallet{
		ID:        walletID,
		Currency:  req.Currency,
		Balance:   100000, // Default $1,000.00 funding balance for demo
		UpdatedAt: now,
	}
	a.store.SaveWallet(wallet)

	card := &cards.Card{
		ID:             uuid.New(),
		WalletID:       walletID,
		CardholderName: req.CardholderName,
		FullPAN:        fullPAN,
		MaskedPAN:      maskedPAN,
		ExpiryMonth:    int(now.Month()),
		ExpiryYear:     now.Year() + 4,
		CVV:            cards.GenerateCVV(),
		Currency:       req.Currency,
		CardType:       cardType,
		Status:         cards.StatusActive,
		SpendingLimits: cards.SpendingLimits{
			PerTransactionCents: req.SpendingLimits.PerTransactionCents,
			DailyCents:          req.SpendingLimits.DailyCents,
			MonthlyCents:        req.SpendingLimits.MonthlyCents,
		},
		MCCWhitelist: req.MccWhitelist,
		MCCBlacklist: req.MccBlacklist,
		CreatedAt:    now,
	}
	a.store.SaveCard(card)

	res := gen.Card{
		Id:             card.ID,
		CardholderName: card.CardholderName,
		MaskedPan:      card.MaskedPAN,
		ExpiryMonth:    card.ExpiryMonth,
		ExpiryYear:     card.ExpiryYear,
		Cvv:            card.CVV,
		Currency:       card.Currency,
		CardType:       gen.CardType(card.CardType),
		Status:         gen.CardStatus(card.Status),
		SpendingLimits: gen.SpendingLimits{
			PerTransactionCents: card.SpendingLimits.PerTransactionCents,
			DailyCents:          card.SpendingLimits.DailyCents,
			MonthlyCents:        card.SpendingLimits.MonthlyCents,
		},
		MccWhitelist: card.MCCWhitelist,
		MccBlacklist: card.MCCBlacklist,
		CreatedAt:    card.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(res)
}

// GetCard handles GET /v1/cards/{cardId}
func (a *CardAPI) GetCard(w http.ResponseWriter, r *http.Request, cardId uuid.UUID) {
	card, err := a.store.GetCard(cardId)
	if err != nil {
		gen.RespondProblem(w, http.StatusNotFound, "CARD_NOT_FOUND", "Card not found")
		return
	}

	res := gen.Card{
		Id:             card.ID,
		CardholderName: card.CardholderName,
		MaskedPan:      card.MaskedPAN,
		ExpiryMonth:    card.ExpiryMonth,
		ExpiryYear:     card.ExpiryYear,
		Cvv:            card.CVV,
		Currency:       card.Currency,
		CardType:       gen.CardType(card.CardType),
		Status:         gen.CardStatus(card.Status),
		SpendingLimits: gen.SpendingLimits{
			PerTransactionCents: card.SpendingLimits.PerTransactionCents,
			DailyCents:          card.SpendingLimits.DailyCents,
			MonthlyCents:        card.SpendingLimits.MonthlyCents,
		},
		MccWhitelist: card.MCCWhitelist,
		MccBlacklist: card.MCCBlacklist,
		CreatedAt:    card.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

// FreezeCard handles POST /v1/cards/{cardId}/freeze
func (a *CardAPI) FreezeCard(w http.ResponseWriter, r *http.Request, cardId uuid.UUID) {
	card, err := a.store.GetCard(cardId)
	if err != nil {
		gen.RespondProblem(w, http.StatusNotFound, "CARD_NOT_FOUND", "Card not found")
		return
	}

	card.Status = cards.StatusFrozen
	a.store.UpdateCard(card)

	res := gen.Card{
		Id:             card.ID,
		CardholderName: card.CardholderName,
		MaskedPan:      card.MaskedPAN,
		ExpiryMonth:    card.ExpiryMonth,
		ExpiryYear:     card.ExpiryYear,
		Cvv:            card.CVV,
		Currency:       card.Currency,
		CardType:       gen.CardType(card.CardType),
		Status:         gen.CardStatus(card.Status),
		SpendingLimits: gen.SpendingLimits{
			PerTransactionCents: card.SpendingLimits.PerTransactionCents,
			DailyCents:          card.SpendingLimits.DailyCents,
			MonthlyCents:        card.SpendingLimits.MonthlyCents,
		},
		CreatedAt: card.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

// UnfreezeCard handles POST /v1/cards/{cardId}/unfreeze
func (a *CardAPI) UnfreezeCard(w http.ResponseWriter, r *http.Request, cardId uuid.UUID) {
	card, err := a.store.GetCard(cardId)
	if err != nil {
		gen.RespondProblem(w, http.StatusNotFound, "CARD_NOT_FOUND", "Card not found")
		return
	}

	card.Status = cards.StatusActive
	a.store.UpdateCard(card)

	res := gen.Card{
		Id:             card.ID,
		CardholderName: card.CardholderName,
		MaskedPan:      card.MaskedPAN,
		ExpiryMonth:    card.ExpiryMonth,
		ExpiryYear:     card.ExpiryYear,
		Cvv:            card.CVV,
		Currency:       card.Currency,
		CardType:       gen.CardType(card.CardType),
		Status:         gen.CardStatus(card.Status),
		SpendingLimits: gen.SpendingLimits{
			PerTransactionCents: card.SpendingLimits.PerTransactionCents,
			DailyCents:          card.SpendingLimits.DailyCents,
			MonthlyCents:        card.SpendingLimits.MonthlyCents,
		},
		CreatedAt: card.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

// ProcessAuthorization handles POST /v1/authorizations/simulate
func (a *CardAPI) ProcessAuthorization(w http.ResponseWriter, r *http.Request) {
	var req gen.AuthorizationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		gen.RespondProblem(w, http.StatusBadRequest, "MALFORMED_JSON", "Invalid JSON payload")
		return
	}

	decision, err := a.authorizer.Authorize(req.CardId, req.AmountCents, req.Currency, req.MerchantName, req.Mcc)
	if err != nil {
		gen.RespondProblem(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	var fxRatePtr *float64
	if decision.FXRate != 1.0 {
		fxRatePtr = &decision.FXRate
	}

	var declineReasonPtr *string
	if decision.DeclineReason != "" {
		declineReasonPtr = &decision.DeclineReason
	}

	res := gen.AuthorizationResponse{
		Id:                 decision.ID,
		Decision:           decision.Decision,
		IsoResponseCode:    decision.ISOResponseCode,
		DeclineReason:      declineReasonPtr,
		SettledAmountCents: decision.SettledAmountCents,
		SettledCurrency:    decision.SettledCurrency,
		FxRate:             fxRatePtr,
		LatencyMs:          decision.LatencyMs,
		CreatedAt:          decision.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

// RequestFXQuote handles POST /v1/fx/quotes
func (a *CardAPI) RequestFXQuote(w http.ResponseWriter, r *http.Request) {
	var req gen.CreateFXQuoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		gen.RespondProblem(w, http.StatusBadRequest, "MALFORMED_JSON", "Invalid JSON payload")
		return
	}

	quote, err := a.fxEngine.CreateGuaranteedQuote(req.SourceCurrency, req.TargetCurrency, req.AmountCents, 50)
	if err != nil {
		gen.RespondProblem(w, http.StatusBadRequest, "QUOTE_ERROR", err.Error())
		return
	}

	res := gen.FXQuote{
		QuoteId:              quote.ID,
		SourceCurrency:       quote.SourceCurrency,
		TargetCurrency:       quote.TargetCurrency,
		Rate:                 quote.Rate,
		SpreadBps:            quote.SpreadBps,
		OriginalAmountCents:  quote.OriginalAmountCents,
		ConvertedAmountCents: quote.ConvertedAmountCents,
		ExpiresAt:            quote.ExpiresAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(res)
}

// CreateProposal handles POST /v1/ops/maker-checker/proposals
func (a *CardAPI) CreateProposal(w http.ResponseWriter, r *http.Request) {
	var req gen.CreateProposalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		gen.RespondProblem(w, http.StatusBadRequest, "MALFORMED_JSON", "Invalid JSON payload")
		return
	}

	p, err := a.compliance.CreateProposal(req.Action, req.TargetCardId, req.ProposedLimitCents, req.MakerId, req.Rationale)
	if err != nil {
		gen.RespondProblem(w, http.StatusBadRequest, "PROPOSAL_FAILED", err.Error())
		return
	}

	res := gen.Proposal{
		Id:                 p.ID,
		Action:             p.Action,
		TargetCardId:       p.TargetCardID,
		ProposedLimitCents: p.ProposedLimitCents,
		Status:             p.Status,
		MakerId:            p.MakerID,
		Rationale:          p.Rationale,
		CreatedAt:          p.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(res)
}

// ReviewProposal handles POST /v1/ops/maker-checker/proposals/{proposalId}/approve
func (a *CardAPI) ReviewProposal(w http.ResponseWriter, r *http.Request, proposalId uuid.UUID) {
	var req gen.ReviewProposalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		gen.RespondProblem(w, http.StatusBadRequest, "MALFORMED_JSON", "Invalid JSON payload")
		return
	}

	p, err := a.compliance.ReviewProposal(proposalId, req.CheckerId, req.Approved, req.ReviewNotes)
	if err != nil {
		if errors.Is(err, compliance.ErrMakerCannotApproveSelf) {
			gen.RespondProblem(w, http.StatusForbidden, "MAKER_CHECKER_VIOLATION", err.Error())
			return
		}
		gen.RespondProblem(w, http.StatusBadRequest, "REVIEW_FAILED", err.Error())
		return
	}

	res := gen.Proposal{
		Id:                 p.ID,
		Action:             p.Action,
		TargetCardId:       p.TargetCardID,
		ProposedLimitCents: p.ProposedLimitCents,
		Status:             p.Status,
		MakerId:            p.MakerID,
		CheckerId:          p.CheckerID,
		Rationale:          p.Rationale,
		ReviewNotes:        p.ReviewNotes,
		CreatedAt:          p.CreatedAt,
		ReviewedAt:         p.ReviewedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}
