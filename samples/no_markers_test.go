package sample_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCalculateTotalWithoutMarkers(t *testing.T) {
	t.Run("when markers are absent for two coffee items, total price 1000 is returned", func(t *testing.T) {
		// Arrange
		items := []item{{name: "coffee", price: 500, count: 2}}

		// Act
		got := calculateTotal(items)

		// Assert
		assert.Equal(t, 1000, got)
	})
}
