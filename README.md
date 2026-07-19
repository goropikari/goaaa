# goaaa

Go のテストコードに書かれた `Arrange`・`Act`・`Assert` マーカーの順序を検査する CLI linter です。

## インストール

```bash
go install github.com/goropikari/goaaa/cmd/aaago@latest
```

## 使い方

ファイルまたはディレクトリを指定します。

```bash
aaago ./path/to/example_test.go
aaago ./path/to/package
```

変更された Go ファイルだけを検査する場合は `--diff` を使います。これは作業ツリーに対する `git diff` の変更ファイルを対象にします。

```bash
aaago --diff
```

SARIF 形式で出力することもできます。

```bash
aaago --format sarif ./path/to/package > results.sarif
```

## 検査ルール

次のコメントをフェーズマーカーとして認識します。大文字・小文字は区別せず、補足説明も付けられます。

```go
// Arrange: input setup
// Act
// Assert: expected result
```

フェーズは `Arrange → Act → Assert` の順でなければなりません。同じフェーズのマーカーは複数回書けます。マーカーがないテストは判定対象になりません。

`TestXxx` と `t.Run` のコールバックは、それぞれ独立したスコープとして検査します。マーカーから意味を判定できない処理や、マーカーのないテストについては警告しません。

## 終了コード

- `0`: 違反なし
- `1`: AAA 順序違反あり
- `2`: 構文エラー、入力エラー、オプションエラーなど

テキスト形式の診断は標準エラーへ、SARIF は標準出力へ出力します。診断にはファイル、行番号、列番号、ルール ID `AAA001` が含まれます。

## サンプル

意図的な順序違反を含むサンプルを解析できます。

```bash
go run ./cmd/aaago ./samples
```

マーカーを書かない通常のテストは [samples/no_markers_test.go](samples/no_markers_test.go) にあります。

## 開発

```bash
go test ./...
go vet ./...
make fmt
make lint
```

`make lint` は golangci-lint によるコード検査です。
