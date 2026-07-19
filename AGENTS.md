# 開発ガイドライン

## テスト

- テストは AAA pattern（Arrange・Act・Assert）で記述する。テスト対象の準備、実行、結果の検証を分け、各段階が読み取れる構成にする。
- アサーションには `github.com/stretchr/testify` を使う。`assert` や `require` で、失敗時に意図が分かる検証を書く。
- テスト関数名にはテスト対象の関数名を含める。たとえば `CalculateTotal` のテストは `TestCalculateTotal` とする。
- 複数のケースを検証する場合は `t.Run` を使う。
- `t.Run` の説明には、そのケースの前提条件と期待値を書く。入力値だけでなく、どの条件で何が起きるべきかが分かる名前にする。

```go
import "github.com/stretchr/testify/assert"

func TestCalculateTotal(t *testing.T) {
	tests := []struct {
		name string
		input []int
		want int
	}{
		{
			name:  "商品が2件あるとき合計金額が返る",
			input: []int{100, 200},
			want:  300,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			input := tt.input

			// Act
			got := CalculateTotal(input)

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}
```
