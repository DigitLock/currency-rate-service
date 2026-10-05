package polling

import "testing"

func TestPgNumericFromString(t *testing.T) {
	tests := []struct {
		in      string
		wantInt int64
		wantExp int32
	}{
		{in: "1.3225", wantInt: 13225, wantExp: -4},
		{in: "0.0095711601", wantInt: 95711601, wantExp: -10},
		{in: "2500", wantInt: 2500, wantExp: 0},
		{in: "0.000015", wantInt: 15, wantExp: -6},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := pgNumericFromString(tt.in)
			if err != nil {
				t.Fatalf("pgNumericFromString(%q) error: %v", tt.in, err)
			}
			if !got.Valid {
				t.Error("Valid = false, want true")
			}
			if got.Int.Int64() != tt.wantInt || got.Exp != tt.wantExp {
				t.Errorf("got Int=%s Exp=%d, want Int=%d Exp=%d", got.Int, got.Exp, tt.wantInt, tt.wantExp)
			}
		})
	}
}

func TestPgNumericFromStringInvalid(t *testing.T) {
	for _, in := range []string{"abc", "", "-1.5", "1e5", "1.2.3"} {
		if _, err := pgNumericFromString(in); err == nil {
			t.Errorf("pgNumericFromString(%q): expected error, got nil", in)
		}
	}
}
