package domain

import (
	"encoding/json"
	"strings"

	"github.com/gocanto/money/currency"
	gmoney "github.com/gocanto/money/money"
)

type Money struct {
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
}

func NewMoney(amount int64, code string) (Money, error) {
	code = strings.ToUpper(strings.TrimSpace(code))

	if amount <= 0 {
		return Money{}, ErrInvalidAmount
	}

	if !SupportedCurrency(code) {
		return Money{}, ErrInvalidCurrency
	}

	m := gmoney.NewManager().Create(amount, code)
	gotAmount, err := m.Amount()

	if err != nil {
		return Money{}, err
	}

	gotCurrency, err := m.Currency()

	if err != nil {
		return Money{}, err
	}

	return Money{Amount: gotAmount, Currency: gotCurrency.Code}, nil
}

func SupportedCurrency(code string) bool {
	switch strings.ToUpper(strings.TrimSpace(code)) {
	case currency.USD, currency.GEL:
		return true
	default:
		return false
	}
}

func (m Money) Validate() error {
	_, err := NewMoney(m.Amount, m.Currency)

	return err
}

func (m Money) toLibraryMoney() *gmoney.Money {
	return gmoney.NewManager().Create(m.Amount, strings.ToUpper(m.Currency))
}

func (m Money) MarshalJSON() ([]byte, error) {
	type payload Money

	return json.Marshal(payload{
		Amount:   m.Amount,
		Currency: strings.ToUpper(m.Currency),
	})
}
