package odds

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

const minAmericanMagnitude = 100

var (
	ErrInvalidAmerican    = errors.New("invalid american odds")
	ErrInvalidDecimal     = errors.New("invalid decimal odds")
	ErrInvalidProbability = errors.New("probability must be strictly between 0 and 1")
)

type Decimal float64

func NewDecimal(value float64) (Decimal, error) {
	if value <= 1 || math.IsInf(value, 0) || math.IsNaN(value) {
		return 0, fmt.Errorf("%w: %v", ErrInvalidDecimal, value)
	}
	return Decimal(value), nil
}

func (d Decimal) American() American {
	if d >= 2 {
		return American(math.Round((float64(d) - 1) * 100))
	}
	return American(math.Round(-100 / (float64(d) - 1)))
}

func (d Decimal) ImpliedProbability() float64 {
	return 1 / float64(d)
}

func (d Decimal) String() string {
	return strconv.FormatFloat(float64(d), 'g', -1, 64)
}

type American int

func NewAmerican(value int) (American, error) {
	if value > -minAmericanMagnitude && value < minAmericanMagnitude {
		return 0, fmt.Errorf("%w: %d", ErrInvalidAmerican, value)
	}
	return American(value), nil
}

func ParseAmerican(text string) (American, error) {
	value, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil {
		return 0, fmt.Errorf("%w: %q", ErrInvalidAmerican, text)
	}
	return NewAmerican(value)
}

func AmericanFromProbability(probability float64) (American, error) {
	if probability <= 0 || probability >= 1 {
		return 0, fmt.Errorf("%w: %v", ErrInvalidProbability, probability)
	}
	if probability >= 0.5 {
		return American(math.Round(-100 * probability / (1 - probability))), nil
	}
	return American(math.Round(100 * (1 - probability) / probability)), nil
}

func (a American) Decimal() Decimal {
	if a > 0 {
		return Decimal(1 + float64(a)/100)
	}
	return Decimal(1 + 100/float64(-a))
}

func (a American) String() string {
	if a > 0 {
		return "+" + strconv.Itoa(int(a))
	}
	return strconv.Itoa(int(a))
}
