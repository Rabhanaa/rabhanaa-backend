package service

import (
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
)

// decimalToNumeric converts a decimal to pgtype.Numeric preserving the fractional
// part. Building the value from BigInt() instead truncates to a whole number and
// leaves Exp at zero, which silently drops piastres from every price.
func decimalToNumeric(d decimal.Decimal) pgtype.Numeric {
	return pgtype.Numeric{Int: d.Coefficient(), Exp: d.Exponent(), Valid: true}
}

func numericToString(n pgtype.Numeric) string {
	if !n.Valid || n.Int == nil {
		return "0"
	}
	return decimal.NewFromBigInt(n.Int, n.Exp).String()
}

// moderationReasonFor returns the admin's reason only to the post's owner —
// other users have no business seeing why a listing was refused.
func moderationReasonFor(reason pgtype.Text, isOwner bool) *string {
	if !isOwner || !reason.Valid || reason.String == "" {
		return nil
	}
	return &reason.String
}
