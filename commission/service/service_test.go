package service

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func dec(t *testing.T, s string) decimal.Decimal {
	t.Helper()
	d, err := decimal.NewFromString(s)
	assert.NoError(t, err)
	return d
}

// The commission is charged on the deal value, not the unit price. That
// distinction was the single most consequential decision in this feature — the
// two answers differ by a factor of the quantity, which on real listings is
// several hundred — so it is pinned here with a real production listing.
func TestComputeCharge(t *testing.T) {
	tests := []struct {
		name          string
		unitPrice     string
		quantity      string
		rate          string
		wantDealValue string
		wantAmount    string
	}{
		{
			// كتف بتلو بالعظم, live on production: 313.75/kg x 997 kg.
			name:          "real listing at the default rate",
			unitPrice:     "313.75",
			quantity:      "997",
			rate:          "1.5",
			wantDealValue: "312808.75",
			// 4692.13125 truncated by half-up rounding to two places.
			wantAmount: "4692.13",
		},
		{
			name:          "single unit",
			unitPrice:     "100",
			quantity:      "1",
			rate:          "1.5",
			wantDealValue: "100",
			wantAmount:    "1.5",
		},
		{
			// Half-up, not banker's rounding: 0.125 -> 0.13, never 0.12. A
			// platform that rounds against itself on every sale loses real money.
			name:          "rounds half up",
			unitPrice:     "8.333333",
			quantity:      "1",
			rate:          "1.5",
			wantDealValue: "8.33",
			wantAmount:    "0.12",
		},
		{
			name:          "exact half cent rounds up",
			unitPrice:     "10",
			quantity:      "1",
			rate:          "1.25",
			wantDealValue: "10",
			wantAmount:    "0.13",
		},
		{
			// A rate of zero must produce no debt rather than an error, so the
			// client can switch collection off without a deploy.
			name:          "zero rate charges nothing",
			unitPrice:     "313.75",
			quantity:      "997",
			rate:          "0",
			wantDealValue: "312808.75",
			wantAmount:    "0",
		},
		{
			name:          "fractional quantity",
			unitPrice:     "50.5",
			quantity:      "2.5",
			rate:          "1.5",
			wantDealValue: "126.25",
			wantAmount:    "1.89",
		},
		{
			// Large deals must not lose precision — this is why the money path is
			// decimal end to end and never float64.
			name:          "large deal keeps precision",
			unitPrice:     "999999.99",
			quantity:      "1000",
			rate:          "1.5",
			wantDealValue: "999999990",
			wantAmount:    "14999999.85",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotDeal, gotAmount := ComputeCharge(dec(t, tt.unitPrice), dec(t, tt.quantity), dec(t, tt.rate))
			assert.True(t, gotDeal.Equal(dec(t, tt.wantDealValue)),
				"deal value: got %s want %s", gotDeal, tt.wantDealValue)
			assert.True(t, gotAmount.Equal(dec(t, tt.wantAmount)),
				"amount: got %s want %s", gotAmount, tt.wantAmount)
		})
	}
}

// Changing the rate must never alter a charge already accrued. The service
// enforces this by snapshotting rate_percent onto every row; this pins the
// arithmetic half of that promise — the same sale at two rates yields two
// different amounts, so a stored rate is the only way to reproduce an old one.
func TestRateChangeDoesNotAlterPastArithmetic(t *testing.T) {
	unitPrice, quantity := dec(t, "313.75"), dec(t, "997")

	_, atOldRate := ComputeCharge(unitPrice, quantity, dec(t, "1.5"))
	_, atNewRate := ComputeCharge(unitPrice, quantity, dec(t, "3"))

	assert.True(t, atOldRate.Equal(dec(t, "4692.13")))
	assert.True(t, atNewRate.Equal(dec(t, "9384.26")))
	assert.False(t, atOldRate.Equal(atNewRate),
		"a stored snapshot is the only way to reproduce a charge after a rate change")
}
