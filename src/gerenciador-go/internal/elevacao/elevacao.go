// Package elevacao reabre o próprio gerenciador como administrador (UAC) só para uma ação, e espera terminar, como o
// AcoesElevadas.cs: o resto do gerenciador roda sem elevação.
package elevacao

// Resultado de uma ação elevada.
type Resultado int

const (
	Feito Resultado = iota
	Cancelado
	Falhou
)
