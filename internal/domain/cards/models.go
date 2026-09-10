package cards

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"
)

type CardStatus string

const (
	StatusActive     CardStatus = "ACTIVE"
	StatusFrozen     CardStatus = "FROZEN"
	StatusTerminated CardStatus = "TERMINATED"
)

type CardType string

const (
	TypeVirtual  CardType = "VIRTUAL"
	TypePhysical CardType = "PHYSICAL"
)

type SpendingLimits struct {
	PerTransactionCents int64 `json:"per_transaction_cents"`
	DailyCents          int64 `json:"daily_cents"`
	MonthlyCents        int64 `json:"monthly_cents"`
}

type Card struct {
	ID             uuid.UUID      `json:"id"`
	WalletID       uuid.UUID      `json:"wallet_id"`
	CardholderName string         `json:"cardholder_name"`
	FullPAN        string         `json:"-"` // Stored securely
	MaskedPAN      string         `json:"masked_pan"`
	ExpiryMonth    int            `json:"expiry_month"`
	ExpiryYear     int            `json:"expiry_year"`
	CVV            string         `json:"cvv"`
	Currency       string         `json:"currency"`
	CardType       CardType       `json:"card_type"`
	Status         CardStatus     `json:"status"`
	SpendingLimits SpendingLimits `json:"spending_limits"`
	DailySpend     int64          `json:"daily_spend"`
	MonthlySpend   int64          `json:"monthly_spend"`
	LastSpendReset time.Time      `json:"last_spend_reset"`
	MCCWhitelist   []string       `json:"mcc_whitelist,omitempty"`
	MCCBlacklist   []string       `json:"mcc_blacklist,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
}

// GeneratePAN generates a 16-digit PAN starting with BIN 411111 with valid Luhn checksum
func GeneratePAN() (string, string) {
	n, _ := rand.Int(rand.Reader, big.NewInt(9000000000))
	middle := fmt.Sprintf("%010d", n.Int64()+1000000000)
	raw15 := "411111" + middle[:9]

	// Compute Luhn check digit
	checkDigit := calculateLuhnCheckDigit(raw15)
	fullPAN := fmt.Sprintf("%s%d", raw15, checkDigit)
	maskedPAN := fmt.Sprintf("%s******%s", fullPAN[:6], fullPAN[12:])
	return fullPAN, maskedPAN
}

func calculateLuhnCheckDigit(number string) int {
	sum := 0
	alternate := true
	for i := len(number) - 1; i >= 0; i-- {
		n := int(number[i] - '0')
		if alternate {
			n *= 2
			if n > 9 {
				n = (n % 10) + 1
			}
		}
		sum += n
		alternate = !alternate
	}
	return (10 - (sum % 10)) % 10
}

func GenerateCVV() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(900))
	return fmt.Sprintf("%03d", n.Int64()+100)
}
