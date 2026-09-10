package compliance

import (
	"errors"
	"fmt"
	"sync"
	"time"

	gen "github.com/akmalkhaniub/omnicard-fx/internal/generated/api"
	"github.com/akmalkhaniub/omnicard-fx/internal/repository"
	"github.com/google/uuid"
)

var (
	ErrMakerCannotApproveSelf = errors.New("maker-checker violation: maker cannot approve their own proposal")
	ErrProposalNotFound       = errors.New("proposal not found")
	ErrProposalAlreadyReviewed = errors.New("proposal has already been reviewed")
)

type Proposal struct {
	ID                 uuid.UUID
	Action             gen.ProposalAction
	TargetCardID       uuid.UUID
	ProposedLimitCents int64
	Status             gen.ProposalStatus
	MakerID            string
	CheckerID          *string
	Rationale          string
	ReviewNotes        *string
	CreatedAt          time.Time
	ReviewedAt         *time.Time
}

type Service struct {
	mu        sync.RWMutex
	proposals map[uuid.UUID]*Proposal
	store     *repository.Store
}

func NewService(store *repository.Store) *Service {
	return &Service{
		proposals: make(map[uuid.UUID]*Proposal),
		store:     store,
	}
}

func (s *Service) CreateProposal(action gen.ProposalAction, targetCardID uuid.UUID, proposedLimit int64, makerID string, rationale string) (*Proposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Verify card exists
	_, err := s.store.GetCard(targetCardID)
	if err != nil {
		return nil, err
	}

	p := &Proposal{
		ID:                 uuid.New(),
		Action:             action,
		TargetCardID:       targetCardID,
		ProposedLimitCents: proposedLimit,
		Status:             gen.ProposalPendingReview,
		MakerID:            makerID,
		Rationale:          rationale,
		CreatedAt:          time.Now().UTC(),
	}

	s.proposals[p.ID] = p
	return p, nil
}

func (s *Service) ReviewProposal(proposalID uuid.UUID, checkerID string, approved bool, notes string) (*Proposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, exists := s.proposals[proposalID]
	if !exists {
		return nil, ErrProposalNotFound
	}

	if p.Status != gen.ProposalPendingReview {
		return nil, ErrProposalAlreadyReviewed
	}

	// Strict dual control enforcement
	if checkerID == p.MakerID {
		return nil, fmt.Errorf("%w (user %s cannot approve own action)", ErrMakerCannotApproveSelf, checkerID)
	}

	now := time.Now().UTC()
	p.CheckerID = &checkerID
	p.ReviewNotes = &notes
	p.ReviewedAt = &now

	if approved {
		p.Status = gen.ProposalApproved
		// Apply change to card
		card, err := s.store.GetCard(p.TargetCardID)
		if err == nil {
			if p.Action == gen.ActionLimitIncrease {
				card.SpendingLimits.DailyCents = p.ProposedLimitCents
				card.SpendingLimits.PerTransactionCents = p.ProposedLimitCents
				s.store.UpdateCard(card)
			}
		}
	} else {
		p.Status = gen.ProposalRejected
	}

	return p, nil
}
