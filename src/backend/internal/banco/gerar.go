package banco

// O esquema do sqlc e as consultas geradas. O sqlc roda numa versão fixa pelo go run, sem entrar no go.mod.
// Ele precisa de compilador C (cgo) para compilar: no Linux, o gcc; no Windows, o MSYS2. Só precisa quem muda
// uma consulta ou o esquema; o código gerado vai no commit.
//go:generate go run ./esquema/gerar
//go:generate go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate -f ../../sqlc.yaml
