// Pacote bancos é o BancoService do C# (ADR 0024): o catálogo de bancos que a tela de Contas sabe identificar e o banco
// que vale para cada conta (o escolhido, o da conta do cartão ou o detectado pelos nomes).
package bancos

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco/consultas"
	"golang.org/x/text/unicode/norm"
)

// Outro é o "Outro (sem banco)": escolhido para não identificar banco nenhum (sem selo de marca).
const Outro = "outro"

// Definicao é um banco do catálogo. Os códigos são os de compartilhado/bancos.ts no front-end, que desenha o selo de
// cada um: banco novo entra nos dois lugares (e no C#, enquanto ele existir).
type Definicao struct {
	Codigo, Nome string
	Termos       []string
}

// Catalogo, na ordem do C#: a detecção fica com o primeiro que bate.
var Catalogo = []Definicao{
	{"nubank", "Nubank", []string{"nubank", "nu pagamentos", "nu financeira", "nuconta", "ultravioleta"}},
	{"itau", "Itaú", []string{"itau", "itaucard", "unibanco", "personnalite", "uniclass"}},
	{"mercadopago", "Mercado Pago", []string{"mercado pago", "mercadopago", "mercado credito"}},
	{"inter", "Inter", []string{"banco inter", "inter"}},
	{"bradesco", "Bradesco", []string{"bradesco", "bradescard"}},
	{"santander", "Santander", []string{"santander"}},
	{"bb", "Banco do Brasil", []string{"banco do brasil", "bb", "ourocard"}},
	{"caixa", "Caixa", []string{"caixa", "caixa economica", "cef"}},
	{"c6", "C6 Bank", []string{"c6", "c6 bank", "c6bank"}},
	{"picpay", "PicPay", []string{"picpay"}},
	{"pagbank", "PagBank", []string{"pagbank", "pagseguro"}},
	{"btg", "BTG Pactual", []string{"btg"}},
	{"xp", "XP", []string{"xp", "xp investimentos"}},
	{"neon", "Neon", []string{"neon"}},
	{"next", "Next", []string{"next"}},
	{"will", "Will Bank", []string{"will bank", "willbank"}},
	{"sicoob", "Sicoob", []string{"sicoob"}},
	{"sicredi", "Sicredi", []string{"sicredi"}},
	{"banrisul", "Banrisul", []string{"banrisul"}},
	{"safra", "Safra", []string{"safra"}},
	{"original", "Banco Original", []string{"banco original"}},
	{"pan", "Banco Pan", []string{"banco pan"}},
	{"bmg", "BMG", []string{"bmg"}},
	{"agibank", "Agibank", []string{"agibank"}},
	{"nomad", "Nomad", []string{"nomad"}},
	{"wise", "Wise", []string{"wise"}},
	{"revolut", "Revolut", []string{"revolut"}},
	{"paypal", "PayPal", []string{"paypal"}},
	{Outro, "Outro (sem banco)", nil},
}

// Existe diz se o código é de um banco do catálogo (o "outro" inclusive).
func Existe(codigo *string) bool { return Nome(codigo) != nil }

// Nome é o nome de exibição do código, ou nil quando ele não está no catálogo.
func Nome(codigo *string) *string {
	if codigo == nil {
		return nil
	}
	for _, b := range Catalogo {
		if b.Codigo == *codigo {
			nome := b.Nome
			return &nome
		}
	}
	return nil
}

// Detectar devolve o primeiro banco do catálogo com um termo que aparece como palavra inteira, sem acento nem
// maiúscula, em algum dos textos (na ordem: o nome da instituição antes do nome da conta); nil quando nenhum bate.
//
// No C# é a regex \b(termos)\b sobre o texto normalizado. Depois de normalizado, ele só tem letras, dígitos e um espaço
// entre as palavras, e a fronteira de palavra só pode estar num espaço ou nas pontas: o termo entre espaços é o mesmo.
// O \b do Go seria só ASCII, e o do .NET vale para qualquer letra.
func Detectar(textos ...*string) *string {
	for _, texto := range textos {
		if texto == nil {
			continue
		}
		normalizado := " " + normalizar(*texto) + " "
		for _, b := range Catalogo {
			for _, termo := range b.Termos {
				if strings.Contains(normalizado, " "+termo+" ") {
					codigo := b.Codigo
					return &codigo
				}
			}
		}
	}
	return nil
}

// normalizar é o Normalizar do BancoService: decompõe (FormD), descarta as marcas (Mn), troca o que não é letra nem
// dígito por espaço, passa para minúscula (ToLowerInvariant), junta os espaços e apara. O C# olha um char (UTF-16) por
// vez: o caractere fora do plano básico, como o "𝐧" matemático, chega em duas metades que não são letra, e vira espaço.
func normalizar(texto string) string {
	var sb strings.Builder
	for _, r := range norm.NFD.String(texto) {
		switch {
		case r > 0xFFFF:
			sb.WriteByte(' ')
		case unicode.Is(unicode.Mn, r):
		case unicode.IsLetter(r) || unicode.Is(unicode.Nd, r):
			sb.WriteRune(unicode.ToLower(r))
		default:
			sb.WriteByte(' ')
		}
	}
	return strings.Join(strings.Fields(sb.String()), " ")
}

// ContaParaBanco é o que a identificação do banco precisa de cada conta.
type ContaParaBanco struct {
	Id               int64
	Nome             string
	NomeInstituicao  *string
	Banco            *string
	ContaVinculadaId *int64
}

// Efetivos é o banco de cada conta: o escolhido; senão, para o cartão, o da conta vinculada (escolhido ou detectado);
// senão, o detectado pelos nomes. "Outro" vira nil (sem banco), mas impede a detecção.
func Efetivos(contas []ContaParaBanco) map[int64]*string {
	porId := make(map[int64]ContaParaBanco, len(contas))
	for _, c := range contas {
		porId[c.Id] = c
	}
	proprio := func(c ContaParaBanco) *string {
		if c.Banco != nil {
			return c.Banco
		}
		return Detectar(c.NomeInstituicao, &c.Nome)
	}
	resultado := make(map[int64]*string, len(contas))
	for _, c := range contas {
		b := c.Banco
		if b == nil && c.ContaVinculadaId != nil {
			if vinculada, ok := porId[*c.ContaVinculadaId]; ok {
				b = proprio(vinculada)
			}
		}
		if b == nil {
			b = proprio(c)
		}
		if b != nil && *b == Outro {
			b = nil
		}
		resultado[c.Id] = b
	}
	return resultado
}

// Carregar é o BancosAsync do ContaService: o banco que vale para cada conta, com o nome da instituição da conexão.
func Carregar(ctx context.Context, q consultas.DBTX) (map[int64]*string, error) {
	linhas, err := q.QueryContext(ctx, `SELECT c."Id", c."Nome", x."NomeInstituicao", c."Banco", c."ContaVinculadaId"
		FROM "Conta" AS c LEFT JOIN "ConexaoBancaria" AS x ON c."ConexaoBancariaId" = x."Id"`)
	if err != nil {
		return nil, fmt.Errorf("ler as contas para o banco: %w", err)
	}
	defer linhas.Close()
	var contas []ContaParaBanco
	for linhas.Next() {
		var c ContaParaBanco
		if err := linhas.Scan(&c.Id, &c.Nome, &c.NomeInstituicao, &c.Banco, &c.ContaVinculadaId); err != nil {
			return nil, fmt.Errorf("ler conta para o banco: %w", err)
		}
		contas = append(contas, c)
	}
	if err := linhas.Err(); err != nil {
		return nil, fmt.Errorf("ler as contas para o banco: %w", err)
	}
	return Efetivos(contas), nil
}
