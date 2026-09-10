package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/akmalkhaniub/omnicard-fx/internal/api"
	"github.com/akmalkhaniub/omnicard-fx/internal/domain/fx"
	gen "github.com/akmalkhaniub/omnicard-fx/internal/generated/api"
	"github.com/akmalkhaniub/omnicard-fx/internal/repository"
	"github.com/go-chi/chi/v5"
)

func setupTestServer() http.Handler {
	store := repository.NewStore()
	fxEngine := fx.NewEngine()
	cardAPI := api.NewCardAPI(store, fxEngine)

	r := chi.NewRouter()
	gen.HandlerFromMux(cardAPI, r)
	return r
}

func TestE2E_CardIssuingAndJITLifecycle(t *testing.T) {
	handler := setupTestServer()

	// 1. Issue Card via POST /v1/cards
	issueReq := gen.IssueCardRequest{
		CardholderName: "Alex Mercer",
		Currency:       "USD",
		SpendingLimits: gen.SpendingLimits{
			PerTransactionCents: 50000,  // $500.00
			DailyCents:          200000, // $2,000.00
			MonthlyCents:        1000000,
		},
		MccBlacklist: []string{"7995"},
	}
	bodyBytes, _ := json.Marshal(issueReq)
	req, _ := http.NewRequest(http.MethodPost, "/v1/cards", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}

	var card gen.Card
	_ = json.Unmarshal(rec.Body.Bytes(), &card)

	if card.MaskedPan == "" || card.Cvv == "" {
		t.Errorf("expected generated PAN and CVV, got %+v", card)
	}

	// 2. Simulate Authorization via POST /v1/authorizations/simulate
	authReq := gen.AuthorizationRequest{
		CardId:       card.Id,
		AmountCents:  1500, // $15.00
		Currency:     "USD",
		MerchantName: "Starbucks Coffee",
		Mcc:          "5814",
	}
	authBytes, _ := json.Marshal(authReq)
	reqAuth, _ := http.NewRequest(http.MethodPost, "/v1/authorizations/simulate", bytes.NewBuffer(authBytes))
	reqAuth.Header.Set("Content-Type", "application/json")

	recAuth := httptest.NewRecorder()
	handler.ServeHTTP(recAuth, reqAuth)

	if recAuth.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for auth, got %d: %s", recAuth.Code, recAuth.Body.String())
	}

	var authRes gen.AuthorizationResponse
	_ = json.Unmarshal(recAuth.Body.Bytes(), &authRes)

	if authRes.Decision != "APPROVED" || authRes.IsoResponseCode != "00" {
		t.Errorf("expected APPROVED (00), got %s (%s)", authRes.Decision, authRes.IsoResponseCode)
	}

	// 3. Freeze Card via POST /v1/cards/{cardId}/freeze
	freezeReq, _ := http.NewRequest(http.MethodPost, "/v1/cards/"+card.Id.String()+"/freeze", nil)
	freezeRec := httptest.NewRecorder()
	handler.ServeHTTP(freezeRec, freezeReq)

	if freezeRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for freeze, got %d", freezeRec.Code)
	}

	// 4. Try auth on frozen card -> MUST DECLINE (57)
	reqAuthFrozen, _ := http.NewRequest(http.MethodPost, "/v1/authorizations/simulate", bytes.NewBuffer(authBytes))
	reqAuthFrozen.Header.Set("Content-Type", "application/json")
	recAuthFrozen := httptest.NewRecorder()
	handler.ServeHTTP(recAuthFrozen, reqAuthFrozen)

	var authFrozenRes gen.AuthorizationResponse
	_ = json.Unmarshal(recAuthFrozen.Body.Bytes(), &authFrozenRes)

	if authFrozenRes.Decision != "DECLINED" || authFrozenRes.IsoResponseCode != "57" {
		t.Errorf("expected DECLINED (57) for frozen card, got %s (%s)", authFrozenRes.Decision, authFrozenRes.IsoResponseCode)
	}

	// 5. Test Maker-Checker Proposal & Review
	propReq := gen.CreateProposalRequest{
		Action:             gen.ActionLimitIncrease,
		TargetCardId:       card.Id,
		ProposedLimitCents: 5000000,
		MakerId:            "sarah_admin",
		Rationale:          "Client scale expansion",
	}
	propBytes, _ := json.Marshal(propReq)
	reqProp, _ := http.NewRequest(http.MethodPost, "/v1/ops/maker-checker/proposals", bytes.NewBuffer(propBytes))
	reqProp.Header.Set("Content-Type", "application/json")

	recProp := httptest.NewRecorder()
	handler.ServeHTTP(recProp, reqProp)

	var proposal gen.Proposal
	_ = json.Unmarshal(recProp.Body.Bytes(), &proposal)

	// Maker attempts self-approval -> 403 Forbidden
	reviewSelfReq := gen.ReviewProposalRequest{
		CheckerId:   "sarah_admin",
		Approved:    true,
		ReviewNotes: "Self approving",
	}
	reviewSelfBytes, _ := json.Marshal(reviewSelfReq)
	reqReviewSelf, _ := http.NewRequest(http.MethodPost, "/v1/ops/maker-checker/proposals/"+proposal.Id.String()+"/approve", bytes.NewBuffer(reviewSelfBytes))
	reqReviewSelf.Header.Set("Content-Type", "application/json")

	recReviewSelf := httptest.NewRecorder()
	handler.ServeHTTP(recReviewSelf, reqReviewSelf)

	if recReviewSelf.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for self-approval, got %d", recReviewSelf.Code)
	}
}
