package backoff

import "time"

// Delay doubles base for each consecutive failure up to ceiling, then keeps half of that
// delay fixed and spreads the other half by randomFraction (0 to 1), so retries from many
// slices don't line up.
func Delay(base, ceiling time.Duration, consecutiveFailures int, randomFraction float64) time.Duration {
	exponential := exponentialDelay(base, ceiling, consecutiveFailures)
	half := exponential / 2
	return half + time.Duration(randomFraction*float64(half))
}

func exponentialDelay(base, ceiling time.Duration, consecutiveFailures int) time.Duration {
	delay := base
	for doubling := 1; doubling < consecutiveFailures && delay < ceiling; doubling++ {
		delay *= 2
	}
	return min(delay, ceiling)
}
