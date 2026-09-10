package tests

import (
	"testing"
	"time"

	"github.com/akmalkhaniub/omnicard-fx/internal/compliance"
	"github.com/akmalkhaniub/omnicard-fx/internal/domain/cards"
	gen "github.com/akmalkhaniub/omnicard-fx/internal/generated/api"
	"github.com/akmalkhaniub/omnicard-fx/internal/repository"
	"github.com/google/uuid"
)

func TestMakerChecker_DualControlEnforcement(t *testing.T) {
	store := repository.NewStore()
	compService := compliance.NewService(store)

	card := &cards.Card{
		ID:             uuid.New(),
		CardholderName: "Enterprise Client",
		Currency:       "USD",
		Status:         cards.StatusActive,
		SpendingLimits: cards.SpendingLimits{
			DailyCents: 500000, // $5,000.00
		},
		CreatedAt: time.Now().UTC(),
	}
	store.SaveCard(card)

	// 1. Maker Sarah proposes raising daily limit to $50,000
	proposal, err := compService.CreateProposal(gen.ActionLimitIncrease, card.ID, 5000000, "ops_sarah", "Client seasonal revenue expansion")
	if err != nil {
		t.Fatalf("failed to create proposal: %v", err)
	}

	if proposal.Status != gen.ProposalPendingReview {
		t.Errorf("expected status PENDING_REVIEW, got %s", proposal.Status)
	}

	// 2. Violation: Maker Sarah attempts to approve her own proposal -> MUST FAIL!
	_, err = compService.ReviewProposal(proposal.ID, "ops_sarah", true, "Self-approval attempt")
	if err == nil {
		t.Fatal("expected dual control violation error when maker approves own proposal, but it succeeded!")
	}

	// 3. Separate Compliance Officer Dan approves the proposal -> MUST SUCCEED!
	approvedProp, err := compService.ReviewProposal(proposal.ID, "compliance_officer_dan", true, "Verified KYC tier and tax returns")
	if err != nil {
		t.Fatalf("failed to approve proposal by valid checker: %v", err)
	}

	if approvedProp.Status != gen.ProposalApproved {
		t.Errorf("expected status APPROVED, got %s", approvedProp.Status)
	}

	// 4. Verify card spending limits were updated automatically
	updatedCard, _ := store.GetCard(card.ID)
	if updatedCard.SpendingLimits.DailyCents != 5000000 {
		t.Errorf("expected daily limit 5000000, got %d", updatedCard.SpendingLimits.DailyCents)
	}
}
