# Fontes conferidas

Para a "🎓 Aula rápida" e para toda afirmação sobre o Go citada como fato. Cada link abaixo respondeu e a âncora existia em **01/11/2026**. Link que não está aqui: abra e confira na hora antes de citar; se ele for útil de novo, acrescente aqui, com a data.

## Guias de estilo

Leia junto com "Onde divergimos" em [`estilo.md`](estilo.md): os guias não sabem das decisões do projeto.

- [Effective Go](https://go.dev/doc/effective_go)
- [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments): [interfaces](https://go.dev/wiki/CodeReviewComments#interfaces), [error strings](https://go.dev/wiki/CodeReviewComments#error-strings), [contexts](https://go.dev/wiki/CodeReviewComments#contexts), [goroutine lifetimes](https://go.dev/wiki/CodeReviewComments#goroutine-lifetimes), [declaring empty slices](https://go.dev/wiki/CodeReviewComments#declaring-empty-slices), [initialisms](https://go.dev/wiki/CodeReviewComments#initialisms)
- [Go Test Comments](https://go.dev/wiki/TestComments): [identify the function](https://go.dev/wiki/TestComments#identify-the-function), [got before want](https://go.dev/wiki/TestComments#got-before-want), [assert libraries](https://go.dev/wiki/TestComments#assert-libraries)
- [Google Go Style Guide: decisions](https://google.github.io/styleguide/go/decisions) e [best practices](https://google.github.io/styleguide/go/best-practices)
- [Go Doc Comments](https://go.dev/doc/comment)
- [Organizing a Go module](https://go.dev/doc/modules/layout#server-project)

## Linguagem e biblioteca padrão

- [Especificação da linguagem](https://go.dev/ref/spec)
- Erros: [Working with Errors in Go 1.13](https://go.dev/blog/go1.13-errors)
- `context`: [Go Concurrency Patterns: Context](https://go.dev/blog/context), [pipelines e cancelamento](https://go.dev/blog/pipelines)
- HTTP: [`net/http.ServeMux`](https://pkg.go.dev/net/http#ServeMux), [rotas do Go 1.22](https://go.dev/blog/routing-enhancements), [`httptest.NewRecorder`](https://pkg.go.dev/net/http/httptest#NewRecorder)
- Banco: [`database/sql.Scanner`](https://pkg.go.dev/database/sql#Scanner), [evitar SQL injection](https://go.dev/doc/database/sql-injection)
- JSON: [`encoding/json.Marshal`](https://pkg.go.dev/encoding/json#Marshal) (fatia `nil` vira `null`)
- Ordem: [`slices.SortStableFunc`](https://pkg.go.dev/slices#SortStableFunc)
- Segredo: [`crypto/subtle.ConstantTimeCompare`](https://pkg.go.dev/crypto/subtle#ConstantTimeCompare)
- Log: [`log/slog`](https://pkg.go.dev/log/slog)

## Testes e diagnóstico

- [`testing/synctest`](https://pkg.go.dev/testing/synctest#Test)
- [Go Fuzzing](https://go.dev/doc/security/fuzz/)
- [Race detector](https://go.dev/doc/articles/race_detector)
- [Diagnóstico (pprof, trace)](https://go.dev/doc/diagnostics)

## Bibliotecas do projeto (ADR 0049)

- [`shopspring/decimal`](https://pkg.go.dev/github.com/shopspring/decimal): [`RoundBank`](https://pkg.go.dev/github.com/shopspring/decimal#Decimal.RoundBank), o arredondamento bancário do `Math.Round` do C#
- [`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite)
- [sqlc](https://docs.sqlc.dev/en/latest/)
