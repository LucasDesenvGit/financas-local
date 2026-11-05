// Package servico é o serviço do sistema que roda o site, como o ServicoDoSite.cs: ler o estado não precisa de
// administrador; ligar e desligar, no Windows, precisam (o gerenciador se reabre elevado, pelo UAC, só para isso).
package servico

// Estado é o EstadoServico do C#.
type Estado int

const (
	NaoInstalado Estado = iota
	Parado
	Ligando
	Rodando
	Desligando
)

// Nome do serviço do Windows (LocalSystem), o mesmo do instalador.
const Nome = "FinancasLocal"

// AcaoValida diz se a ação é uma das que o gerenciador roda elevado: ligar, desligar ou reiniciar.
func AcaoValida(acao string) bool {
	return acao == "ligar" || acao == "desligar" || acao == "reiniciar"
}
