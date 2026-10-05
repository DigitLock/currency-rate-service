package polling

import (
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// pgNumericFromString builds a pgtype.Numeric from a plain decimal string (e.g., "1.3225")
// without a float conversion (SRS 2.4.5).
func pgNumericFromString(s string) (pgtype.Numeric, error) {
	// Remove decimal point and count decimal places
	parts := splitDecimal(s)
	digits := parts.integer + parts.fractional
	exp := int32(-len(parts.fractional))

	if digits == "" || strings.Trim(digits, "0123456789") != "" {
		return pgtype.Numeric{}, fmt.Errorf("invalid decimal %q", s)
	}

	intVal, ok := new(big.Int).SetString(digits, 10)
	if !ok {
		return pgtype.Numeric{}, fmt.Errorf("invalid decimal %q", s)
	}

	return pgtype.Numeric{
		Int:   intVal,
		Exp:   exp,
		Valid: true,
	}, nil
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
