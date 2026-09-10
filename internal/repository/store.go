package repository

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/akmalkhaniub/omnicard-fx/internal/domain/cards"
	"github.com/google/uuid"
)

var (
	ErrCardNotFound   = errors.New("card not found")
	ErrWalletNotFound = errors.New("wallet not found")
)

type Wallet struct {
	ID        uuid.UUID
	Currency  string
	Balance   int64 // Minor units (cents)
	UpdatedAt time.Time
}

type AuthorizationRecord struct {
	ID                 uuid.UUID
	CardID             uuid.UUID
	Decision           string
	ISOResponseCode    string
	DeclineReason      string
	OriginalAmount     int64
	OriginalCurrency   string
	SettledAmountCents int64
	SettledCurrency    string
	FXRate             float64
	MerchantName       string
	MCC                string
	LatencyMs          int64
	CreatedAt          time.Time
}

type Store struct {
	mu             sync.RWMutex
	cards          map[uuid.UUID]*cards.Card
	wallets        map[uuid.UUID]*Wallet
	authorizations map[uuid.UUID]*AuthorizationRecord
}

func NewStore() *Store {
	return &Store{
		cards:          make(map[uuid.UUID]*cards.Card),
		wallets:        make(map[uuid.UUID]*Wallet),
		authorizations: make(map[uuid.UUID]*AuthorizationRecord),
	}
}

func (s *Store) SaveCard(card *cards.Card) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cards[card.ID] = card
}

func (s *Store) GetCard(id uuid.UUID) (*cards.Card, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	c, exists := s.cards[id]
	if !exists {
		return nil, ErrCardNotFound
	}
	copyCard := *c
	return &copyCard, nil
}

func (s *Store) UpdateCard(card *cards.Card) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cards[card.ID] = card
}

func (s *Store) SaveWallet(w *Wallet) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.wallets[w.ID] = w
}

func (s *Store) GetWallet(id uuid.UUID) (*Wallet, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	w, exists := s.wallets[id]
	if !exists {
		return nil, ErrWalletNotFound
	}
	copyWallet := *w
	return &copyWallet, nil
}

func (s *Store) DeductWalletBalance(walletID uuid.UUID, amountCents int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	w, exists := s.wallets[walletID]
	if !exists {
		return ErrWalletNotFound
	}

	if w.Balance < amountCents {
		return fmt.Errorf("insufficient wallet balance: available %d, required %d", w.Balance, amountCents)
	}

	w.Balance -= amountCents
	w.UpdatedAt = time.Now().UTC()
	return nil
}

func (s *Store) SaveAuthorization(auth *AuthorizationRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.authorizations[auth.ID] = auth
}
