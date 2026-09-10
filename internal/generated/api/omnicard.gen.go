// Package api provides primitives to interact with the openapi HTTP API.
// Code generated from api/openapi/v1/omnicard.yaml DO NOT EDIT.
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type CardStatus string

const (
	CardStatusActive     CardStatus = "ACTIVE"
	CardStatusFrozen     CardStatus = "FROZEN"
	CardStatusTerminated CardStatus = "TERMINATED"
)

type CardType string

const (
	CardTypeVirtual  CardType = "VIRTUAL"
	CardTypePhysical CardType = "PHYSICAL"
)

type ProposalAction string

const (
	ActionLimitIncrease ProposalAction = "LIMIT_INCREASE"
	ActionCardOverride  ProposalAction = "CARD_OVERRIDE"
	ActionFeeRefund     ProposalAction = "FEE_REFUND"
)

type ProposalStatus string

const (
	ProposalPendingReview ProposalStatus = "PENDING_REVIEW"
	ProposalApproved      ProposalStatus = "APPROVED"
	ProposalRejected      ProposalStatus = "REJECTED"
)

type SpendingLimits struct {
	PerTransactionCents int64 `json:"per_transaction_cents"`
	DailyCents          int64 `json:"daily_cents"`
	MonthlyCents        int64 `json:"monthly_cents"`
}

type Card struct {
	Id             uuid.UUID      `json:"id"`
	CardholderName string         `json:"cardholder_name"`
	MaskedPan      string         `json:"masked_pan"`
	ExpiryMonth    int            `json:"expiry_month"`
	ExpiryYear     int            `json:"expiry_year"`
	Cvv            string         `json:"cvv"`
	Currency       string         `json:"currency"`
	CardType       CardType       `json:"card_type"`
	Status         CardStatus     `json:"status"`
	SpendingLimits SpendingLimits `json:"spending_limits"`
	MccWhitelist   []string       `json:"mcc_whitelist,omitempty"`
	MccBlacklist   []string       `json:"mcc_blacklist,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
}

type IssueCardRequest struct {
	CardholderName string         `json:"cardholder_name"`
	Currency       string         `json:"currency"`
	CardType       *CardType      `json:"card_type,omitempty"`
	SpendingLimits SpendingLimits `json:"spending_limits"`
	MccWhitelist   []string       `json:"mcc_whitelist,omitempty"`
	MccBlacklist   []string       `json:"mcc_blacklist,omitempty"`
}

type AuthorizationRequest struct {
	CardId       uuid.UUID `json:"card_id"`
	AmountCents  int64     `json:"amount_cents"`
	Currency     string    `json:"currency"`
	MerchantName string    `json:"merchant_name"`
	Mcc          string    `json:"mcc"`
	TerminalId   *string   `json:"terminal_id,omitempty"`
}

type AuthorizationResponse struct {
	Id                 uuid.UUID `json:"id"`
	Decision           string    `json:"decision"` // APPROVED / DECLINED
	IsoResponseCode    string    `json:"iso_response_code"`
	DeclineReason      *string   `json:"decline_reason,omitempty"`
	SettledAmountCents int64     `json:"settled_amount_cents"`
	SettledCurrency    string    `json:"settled_currency"`
	FxRate             *float64  `json:"fx_rate,omitempty"`
	LatencyMs          int64     `json:"latency_ms"`
	CreatedAt          time.Time `json:"created_at"`
}

type CreateFXQuoteRequest struct {
	SourceCurrency string `json:"source_currency"`
	TargetCurrency string `json:"target_currency"`
	AmountCents    int64  `json:"amount_cents"`
}

type FXQuote struct {
	QuoteId              uuid.UUID `json:"quote_id"`
	SourceCurrency       string    `json:"source_currency"`
	TargetCurrency       string    `json:"target_currency"`
	Rate                 float64   `json:"rate"`
	SpreadBps            int       `json:"spread_bps"`
	OriginalAmountCents  int64     `json:"original_amount_cents"`
	ConvertedAmountCents int64     `json:"converted_amount_cents"`
	ExpiresAt            time.Time `json:"expires_at"`
}

type CreateProposalRequest struct {
	Action             ProposalAction `json:"action"`
	TargetCardId       uuid.UUID      `json:"target_card_id"`
	ProposedLimitCents int64          `json:"proposed_limit_cents"`
	MakerId            string         `json:"maker_id"`
	Rationale          string         `json:"rationale"`
}

type ReviewProposalRequest struct {
	CheckerId   string `json:"checker_id"`
	Approved    bool   `json:"approved"`
	ReviewNotes string `json:"review_notes"`
}

type Proposal struct {
	Id                 uuid.UUID      `json:"id"`
	Action             ProposalAction `json:"action"`
	TargetCardId       uuid.UUID      `json:"target_card_id"`
	ProposedLimitCents int64          `json:"proposed_limit_cents"`
	Status             ProposalStatus `json:"status"`
	MakerId            string         `json:"maker_id"`
	CheckerId          *string        `json:"checker_id,omitempty"`
	Rationale          string         `json:"rationale"`
	ReviewNotes        *string        `json:"review_notes,omitempty"`
	CreatedAt          time.Time      `json:"created_at"`
	ReviewedAt         *time.Time     `json:"reviewed_at,omitempty"`
}

type ProblemDetails struct {
	Code   string `json:"code"`
	Detail string `json:"detail"`
	Status int    `json:"status"`
	Title  string `json:"title"`
	Type   string `json:"type"`
}

type ServerInterface interface {
	IssueCard(w http.ResponseWriter, r *http.Request)
	GetCard(w http.ResponseWriter, r *http.Request, cardId uuid.UUID)
	FreezeCard(w http.ResponseWriter, r *http.Request, cardId uuid.UUID)
	UnfreezeCard(w http.ResponseWriter, r *http.Request, cardId uuid.UUID)
	ProcessAuthorization(w http.ResponseWriter, r *http.Request)
	RequestFXQuote(w http.ResponseWriter, r *http.Request)
	CreateProposal(w http.ResponseWriter, r *http.Request)
	ReviewProposal(w http.ResponseWriter, r *http.Request, proposalId uuid.UUID)
}

func HandlerFromMux(si ServerInterface, r chi.Router) http.Handler {
	r.Group(func(r chi.Router) {
		r.Post("/v1/cards", si.IssueCard)
		r.Get("/v1/cards/{cardId}", func(w http.ResponseWriter, r *http.Request) {
			id, err := uuid.Parse(chi.URLParam(r, "cardId"))
			if err != nil {
				RespondProblem(w, http.StatusBadRequest, "INVALID_UUID", "Invalid cardId")
				return
			}
			si.GetCard(w, r, id)
		})
		r.Post("/v1/cards/{cardId}/freeze", func(w http.ResponseWriter, r *http.Request) {
			id, err := uuid.Parse(chi.URLParam(r, "cardId"))
			if err != nil {
				RespondProblem(w, http.StatusBadRequest, "INVALID_UUID", "Invalid cardId")
				return
			}
			si.FreezeCard(w, r, id)
		})
		r.Post("/v1/cards/{cardId}/unfreeze", func(w http.ResponseWriter, r *http.Request) {
			id, err := uuid.Parse(chi.URLParam(r, "cardId"))
			if err != nil {
				RespondProblem(w, http.StatusBadRequest, "INVALID_UUID", "Invalid cardId")
				return
			}
			si.UnfreezeCard(w, r, id)
		})
		r.Post("/v1/authorizations/simulate", si.ProcessAuthorization)
		r.Post("/v1/fx/quotes", si.RequestFXQuote)
		r.Post("/v1/ops/maker-checker/proposals", si.CreateProposal)
		r.Post("/v1/ops/maker-checker/proposals/{proposalId}/approve", func(w http.ResponseWriter, r *http.Request) {
			id, err := uuid.Parse(chi.URLParam(r, "proposalId"))
			if err != nil {
				RespondProblem(w, http.StatusBadRequest, "INVALID_UUID", "Invalid proposalId")
				return
			}
			si.ReviewProposal(w, r, id)
		})
	})
	return r
}

func RespondProblem(w http.ResponseWriter, status int, code string, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ProblemDetails{
		Type:   fmt.Sprintf("https://api.omnicard.io/errors/%s", code),
		Title:  http.StatusText(status),
		Status: status,
		Code:   code,
		Detail: detail,
	})
}
