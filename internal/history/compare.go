package history

import "sort"

// Median returns the middle value of values (the mean of the two middles for an
// even count). An empty slice returns 0.
func Median(values []float64) float64 {
	n := len(values)
	if n == 0 {
		return 0
	}
	s := make([]float64, n)
	copy(s, values)
	sort.Float64s(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}
