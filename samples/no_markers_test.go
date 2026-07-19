package sample_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCalculateTotalWithoutMarkers(t *testing.T) {
	items := []item{{name: "coffee", price: 500, count: 2}}
	got := calculateTotal(items)
	assert.Equal(t, 1000, got)
}
