package odds

import (
	"errors"
	"math"
	"testing"
)

func TestDecimalAmerican(t *testing.T) {
	cases := []struct {
		decimal Decimal
		want    American
	}{
		{decimal: 1.91, want: -110},
		{decimal: 2.00, want: 100},
		{decimal: 2.50, want: 150},
		{decimal: 1.50, want: -200},
	}
	for _, testCase := range cases {
		if got := testCase.decimal.American(); got != testCase.want {
			t.Errorf("Decimal(%v).American() = %v, want %v", testCase.decimal, got, testCase.want)
		}
	}
}

func TestNewDecimalRejectsImpossiblePrices(t *testing.T) {
	for _, value := range []float64{1.0, 0.5, -2, math.NaN(), math.Inf(1)} {
		if _, err := NewDecimal(value); !errors.Is(err, ErrInvalidDecimal) {
			t.Errorf("NewDecimal(%v) error = %v, want ErrInvalidDecimal", value, err)
		}
	}
}

func TestParseAmerican(t *testing.T) {
	cases := map[string]American{"-110": -110, "+150": 150, " 200 ": 200}
	for text, want := range cases {
		got, err := ParseAmerican(text)
		if err != nil {
			t.Fatalf("ParseAmerican(%q) error: %v", text, err)
		}
		if got != want {
			t.Errorf("ParseAmerican(%q) = %v, want %v", text, got, want)
		}
	}
}

func TestParseAmericanRejectsImpossiblePrices(t *testing.T) {
	for _, text := range []string{"-99", "50", "abc", ""} {
		if _, err := ParseAmerican(text); !errors.Is(err, ErrInvalidAmerican) {
			t.Errorf("ParseAmerican(%q) error = %v, want ErrInvalidAmerican", text, err)
		}
	}
}

func TestEvenMoneyReadsBackAsPlusOneHundred(t *testing.T) {
	if got := American(-100).Decimal().American(); got != 100 {
		t.Errorf("-100 read back as %v, want +100", got)
	}
}

func TestFullPrecisionDecimalRoundTripsEveryAmericanPrice(t *testing.T) {
	for value := 101; value <= 5000; value++ {
		for _, price := range []American{American(value), American(-value)} {
			if got := price.Decimal().American(); got != price {
				t.Fatalf("round trip of %v through full-precision decimal = %v", price, got)
			}
		}
	}
}

func TestTwoDecimalRoundingLosesPrecisionOnFavorites(t *testing.T) {
	roundedByProvider := Decimal(math.Round(float64(American(-150).Decimal())*100) / 100)

	if got := roundedByProvider.American(); got != -149 {
		t.Errorf("-150 rounded to %v reads back as %v, want -149", roundedByProvider, got)
	}
}
