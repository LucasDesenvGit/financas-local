package parametros

// Tipo é o TipoParametro do C#. No JSON sai como número (1 ou 2), como o enum do .NET sem JsonStringEnumConverter.
type Tipo int

const (
	Inteiro  Tipo = 1
	Booleano Tipo = 2
)

// Definicao é o DefinicaoParametro: um parâmetro editável pela tela de Parametrização.
type Definicao struct {
	Chave, Grupo, Nome, Descricao string
	Tipo                          Tipo
	Padrao, Minimo, Maximo        int
	Unidade                       *string
	// DoAppsettings diz que o padrão pode vir do appsettings (ChaveAppSettings no C#): Opcoes.Padroes.
	DoAppsettings bool
}

func texto(s string) *string { return &s }

// Catalogo é o ParametroService.Catalogo, na mesma ordem (a da tela). Os textos são copiados do C#: a tela é
// montada a partir deles e o contrato compara.
var Catalogo = []Definicao{
	{Chave: "Sincronizacao.Habilitada", Grupo: "Sincronização", Nome: "Sincronização automática",
		Descricao: "Busca os dados dos bancos sozinha, no intervalo abaixo. Desligada, só sincroniza pelo botão.",
		Tipo:      Booleano, Padrao: 1},
	{Chave: "Sincronizacao.IntervaloHoras", Grupo: "Sincronização", Nome: "Intervalo entre sincronizações",
		Descricao: "De quanto em quanto tempo a sincronização automática roda. O Meu Pluggy atualiza os bancos cerca de uma vez por dia.",
		Tipo:      Inteiro, Padrao: 6, Minimo: 1, Maximo: 48, Unidade: texto("horas")},
	{Chave: "Sincronizacao.DiasRetroativos", Grupo: "Sincronização", Nome: "Dias relidos a cada sincronização",
		Descricao: "Quantos dias para trás são relidos para pegar transações que mudaram ou entraram atrasadas.",
		Tipo:      Inteiro, Padrao: 30, Minimo: 1, Maximo: 365, Unidade: texto("dias")},
	{Chave: "Sincronizacao.DiasPrimeiraCarga", Grupo: "Sincronização", Nome: "Histórico na primeira carga",
		Descricao: "Quantos dias de histórico são buscados quando uma conta é sincronizada pela primeira vez.",
		Tipo:      Inteiro, Padrao: 365, Minimo: 30, Maximo: 1825, Unidade: texto("dias")},

	{Chave: "Painel.DiasProximosVencimentos", Grupo: "Painel", Nome: "Próximos vencimentos",
		Descricao: "Até quantos dias à frente o painel lista as contas a pagar.",
		Tipo:      Inteiro, Padrao: 15, Minimo: 1, Maximo: 90, Unidade: texto("dias")},
	{Chave: "Painel.DiasAvisoConsentimento", Grupo: "Painel", Nome: "Aviso de consentimento expirando",
		Descricao: "Com quantos dias de antecedência o painel avisa que o consentimento do Open Finance vai expirar.",
		Tipo:      Inteiro, Padrao: 30, Minimo: 1, Maximo: 120, Unidade: texto("dias")},

	{Chave: "Seguranca.MaximoTentativasLogin", Grupo: "Segurança", Nome: "Tentativas de login antes do bloqueio",
		Descricao: "Quantas senhas erradas seguidas bloqueiam o acesso temporariamente.",
		Tipo:      Inteiro, Padrao: 5, Minimo: 3, Maximo: 20, Unidade: texto("tentativas"), DoAppsettings: true},
	{Chave: "Seguranca.MinutosBloqueio", Grupo: "Segurança", Nome: "Tempo de bloqueio",
		Descricao: "Por quanto tempo o login fica bloqueado depois do limite de tentativas.",
		Tipo:      Inteiro, Padrao: 15, Minimo: 1, Maximo: 1440, Unidade: texto("minutos"), DoAppsettings: true},

	{Chave: "Orcamento.MetaPoupanca", Grupo: "Orçamentos", Nome: "Meta de poupança",
		Descricao: "Quanto da renda a sugestão de orçamentos reserva para guardar ou investir antes de distribuir o resto (regra 50/30/20: pelo menos 20%).",
		Tipo:      Inteiro, Padrao: 20, Minimo: 0, Maximo: 60, Unidade: texto("% da renda")},
	{Chave: "Orcamento.FolgaEssenciais", Grupo: "Orçamentos", Nome: "Folga nos gastos essenciais",
		Descricao: "Margem que a sugestão acrescenta às categorias essenciais (mercado, contas da casa), porque elas variam de um mês para o outro.",
		Tipo:      Inteiro, Padrao: 5, Minimo: 0, Maximo: 30, Unidade: texto("%")},
	{Chave: "Orcamento.CartaoAtencao", Grupo: "Orçamentos", Nome: "Uso do limite do cartão: atenção",
		Descricao: "A partir deste uso do limite dos cartões, a sugestão aperta as categorias de estilo de vida pagas no cartão.",
		Tipo:      Inteiro, Padrao: 30, Minimo: 5, Maximo: 100, Unidade: texto("% do limite")},
	{Chave: "Orcamento.CartaoAlerta", Grupo: "Orçamentos", Nome: "Uso do limite do cartão: alerta",
		Descricao: "A partir deste uso do limite, a sugestão mostra um alerta forte.",
		Tipo:      Inteiro, Padrao: 70, Minimo: 10, Maximo: 100, Unidade: texto("% do limite")},
	{Chave: "Orcamento.CorteCartao", Grupo: "Orçamentos", Nome: "Corte por uso do cartão",
		Descricao: "Quanto a sugestão reduz as categorias de estilo de vida pagas no cartão quando o uso do limite passa do nível de atenção.",
		Tipo:      Inteiro, Padrao: 10, Minimo: 0, Maximo: 50, Unidade: texto("%")},

	{Chave: "Metas.MargemSeguranca", Grupo: "Metas", Nome: "Margem de segurança",
		Descricao: "Quanto da sobra do mês o assistente de metas deixa livre, para imprevistos, antes de recomendar quanto guardar. Também muda na tela de Metas (\"Guardar X% da sobra\").",
		Tipo:      Inteiro, Padrao: 10, Minimo: 0, Maximo: 50, Unidade: texto("% da sobra")},

	{Chave: "Transacoes.MinimoLetrasDescricao", Grupo: "Transações", Nome: "Descrição pouco informativa",
		Descricao: "Tirando números e palavras genéricas (PIX, TED, PAGAMENTO, nome do banco), a descrição com menos letras que isso entra no filtro \"Descrição pouco informativa\".",
		Tipo:      Inteiro, Padrao: 3, Minimo: 1, Maximo: 20, Unidade: texto("letras")},

	{Chave: "Diagnostico.DiasRetencaoErros", Grupo: "Erros do sistema", Nome: "Guardar erros por",
		Descricao: "Erros e avisos mais antigos que isso (pela última ocorrência) são apagados sozinhos, uma vez por dia.",
		Tipo:      Inteiro, Padrao: 90, Minimo: 7, Maximo: 365, Unidade: texto("dias")},
}

// definicao procura a chave no catálogo (comparação exata, como o d.Chave == chave do C#).
func definicao(chave string) (Definicao, bool) {
	for _, d := range Catalogo {
		if d.Chave == chave {
			return d, true
		}
	}
	return Definicao{}, false
}

// limitar é o Math.Clamp.
func limitar(n, minimo, maximo int) int {
	return max(minimo, min(n, maximo))
}
