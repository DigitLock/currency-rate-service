package polling

import (
	"math/big"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func pgNumericFromFloat(f float64) pgtype.Numeric {
	// Convert float to string to preserve precision, then parse as big.Int with exponent
	s := strconv.FormatFloat(f, 'f', 10, 64)

	// Remove decimal point and count decimal places
	parts := splitDecimal(s)
	digits := parts.integer + parts.fractional
	exp := int32(-len(parts.fractional))

	intVal := new(big.Int)
	intVal.SetString(digits, 10)

	return pgtype.Numeric{
		Int:   intVal,
		Exp:   exp,
		Valid: true,
	}
}

type decimalParts struct {
	integer    string
	fractional string
}

func splitDecimal(s string) decimalParts {
	dotIdx := -1
	for i, c := range s {
		if c == '.' {
			dotIdx = i
			break
		}
	}
	if dotIdx == -1 {
		return decimalParts{integer: s, fractional: ""}
	}
	return decimalParts{
		integer:    s[:dotIdx],
		fractional: s[dotIdx+1:],
	}
}

func pgTimestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{
		Time:  t,
		Valid: true,
	}
}

func pgTextFromString(s string) pgtype.Text {
	return pgtype.Text{
		String: s,
		Valid:  true,
	}
}
