// Pacote poucoinformativa é o DescricaoPoucoInformativaService do C# (#9): a transação com descrição que não diz o que
// foi ("PIX ENVIADO", "PAGAMENTO", "TED 123456", só o nome do banco). Sem palavras úteis, nem as regras nem as sugestões
// conseguem categorizar, e o gasto fica escondido nos relatórios. A tela de Transações (#78) filtra por ela, e o aviso
// do painel (#80) conta quantas há.
package poucoinformativa

import (
	"strings"
	"unicode"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/texto"
	"golang.org/x/text/unicode/norm"
)

// ChaveMinimoLetras é o parâmetro com o mínimo de letras úteis (Configurações › Parâmetros).
const ChaveMinimoLetras = "Transacoes.MinimoLetrasDescricao"

// Candidatas é a condição das que ainda pedem atenção, para o WHERE de uma consulta em "Transacao" com o apelido t:
// sem categoria escolhida à mão, não marcadas como revisadas e fora das transferências lançadas pelo próprio sistema.
const Candidatas = `NOT t."DescricaoRevisada" AND NOT t."CategoriaManual" AND t."TransferenciaGrupo" IS NULL`

// termosGenericos são as palavras que, sozinhas, não dizem o que foi: tipo de operação, nome de banco, conectivos.
var termosGenericos = map[string]bool{}

func init() {
	for _, t := range strings.Fields(`
		PIX ENVIADO ENVIADA RECEBIDO RECEBIDA TRANSFERENCIA TRANSFERENCIAS TRANSF TRANSFER TED DOC TEF PAGAMENTO PAGAMENTOS
		PAGTO PGTO PAG PAGO PAGAR RECEBER COMPRA COMPRAS DEBITO DEB CREDITO CRED CARTAO SAQUE DEPOSITO BOLETO BOLETOS
		OPERACAO LANCAMENTO MOVIMENTACAO TRANSACAO TRANSACOES ESTORNO AGENDADO AGENDADA AGENDAMENTO REALIZADO REALIZADA
		EFETUADO EFETUADA ENVIO RECEBIMENTO AUTORIZADO AUTORIZADA AUT AUTOMATICO AUTOMATICA CONTA CONTAS CORRENTE ENTRE
		MESMA TITULARIDADE FAVORECIDO QR CODE QRCODE CHAVE APP INTERNET BANKING MOBILE TITULO VALOR ELETRONICA ELETRONICO
		OUTROS OUTRAS DIVERSOS DIVERSAS
		BANCO NUBANK NU ITAU UNIBANCO BRADESCO SANTANDER CAIXA ECONOMICA FEDERAL INTER BRASIL BB SICOOB SICREDI ORIGINAL
		NEON PAN BTG PACTUAL XP PICPAY PAGBANK PAGSEGURO MERCADOPAGO SAFRA BMG AGIBANK BANRISUL NEXT WILL C6 BANK SA
		DE DA DO DAS DOS PARA PRA EM NO NA NOS NAS A O AS OS E AO VIA POR COM SEM`) {
		termosGenericos[t] = true
	}
}

// LetrasUteis é o LetrasUteis do C#: as letras das palavras que sobram depois de tirar os acentos (FormD sem as marcas),
// a pontuação, os números soltos e os termos genéricos ("99APP" é uma palavra com 3 letras). O C# olha um char (UTF-16)
// por vez: o caractere fora do plano básico, como o "𝐀" matemático, chega em duas metades que não são letra. O
// gabarito é o testdata/letras.json, gravado pela sonda das contas com o próprio C#.
func LetrasUteis(descricao string) int {
	var sb strings.Builder
	for _, r := range norm.NFD.String(descricao) {
		switch {
		case r > 0xFFFF:
			sb.WriteByte(' ')
		case unicode.Is(unicode.Mn, r):
		case unicode.IsLetter(r) || unicode.Is(unicode.Nd, r):
			sb.WriteRune(r)
		default:
			sb.WriteByte(' ')
		}
	}
	letras := 0
	for _, palavra := range strings.Fields(texto.MaiusculasInvariantes(sb.String())) {
		if termosGenericos[palavra] {
			continue
		}
		for _, r := range palavra {
			if unicode.IsLetter(r) {
				letras++
			}
		}
	}
	return letras
}

// EhPoucoInformativa: menos letras úteis que o mínimo.
func EhPoucoInformativa(descricao string, minimoLetras int) bool {
	return LetrasUteis(descricao) < minimoLetras
}
