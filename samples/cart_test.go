package sample_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type item struct {
	name  string
	price int
	count int
}

func calculateTotal(items []item) int {
	total := 0
	for _, item := range items {
		total += item.price * item.count
	}

	return total
}

func TestCalculateTotal(t *testing.T) {
	tests := []struct {
		name  string
		items []item
		want  int
	}{
		{
			name: "when the cart has two products, returns their total price",
			items: []item{
				{name: "notebook", price: 300, count: 2},
				{name: "pen", price: 120, count: 1},
			},
			want: 720,
		},
		{
			name:  "when the cart is empty, returns zero",
			items: nil,
			want:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.items == nil {
				// Act
				got := calculateTotal(tt.items)
				// Arrange
				want := 0
				// Assert
				assert.Equal(t, want, got)

				return
			}

			// Arrange
			items := tt.items
			// Assert
			assert.NotEmpty(t, items)
			// Act
			got := calculateTotal(items)
			assert.Equal(t, tt.want, got)
		})
	}
}
