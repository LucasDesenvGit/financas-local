package holeritepdf

import (
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/holerite"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/texto"
)

// regra do dicionário de rubricas: as expressões (já normalizadas, como o texto.ParaPalavras deixa) que dão o tipo da
// linha do holerite, de um lado (provento ou desconto). Vale a primeira regra que casa, por isso a mais específica vem
// antes ("DSR S/ HORAS EXTRAS" é DSR, não hora extra).
type regra struct {
	provento bool
	tipo     int64
	termos   []string
}

// dicionario é o dado (ADR 0068): para aumentar, acrescente a expressão na regra do tipo. O teste confere que cada
// expressão já está normalizada e que os nomes comuns de cada tipo caem nele.
var dicionario = []regra{
	// Proventos.
	{true, holerite.OutroProvento, []string{"13", "13º", "13O", "DECIMO TERCEIRO", "FERIAS", "SALARIO FAMILIA",
		"SALARIO MATERNIDADE", "ABONO", "GRATIFICACAO", "GRATIFICACOES", "PLR", "PARTICIPACAO NOS LUCROS", "COMISSAO",
		"COMISSOES", "BONUS", "PREMIO", "PREMIACAO", "AJUDA DE CUSTO", "PERICULOSIDADE", "INSALUBRIDADE", "QUINQUENIO",
		"ANUENIO", "BIENIO", "TRIENIO", "REEMBOLSO"}},
	{true, holerite.TipoDSR, []string{"DSR", "D S R", "RSR", "REPOUSO REMUNERADO", "REPOUSO SEMANAL", "DESCANSO SEMANAL",
		"DESCANSO REMUNERADO"}},
	{true, holerite.HoraExtra, []string{"HORA EXTRA", "HORAS EXTRAS", "HORAS EXTRA", "HORA EXTRAS", "HE", "H E", "HR EXTRA",
		"HRS EXTRAS", "HRS EXTRA", "H EXTRA", "H EXTRAS", "SOBREAVISO"}},
	{true, holerite.AdicionalNoturno, []string{"ADICIONAL NOTURNO", "ADIC NOTURNO", "AD NOTURNO", "ADC NOTURNO", "ADIC NOT",
		"ADICIONAL NOT", "HORA NOTURNA", "HORAS NOTURNAS"}},
	{true, holerite.TipoSalario, []string{"SALARIO", "SAL BASE", "ORDENADO", "HORAS NORMAIS", "DIAS TRABALHADOS",
		"HORAS TRABALHADAS", "VENCIMENTO BASE", "PRO LABORE"}},
	// Descontos. O INSS e o IRRF vêm antes: "INSS S/ 13º" é INSS.
	{false, holerite.TipoInss, []string{"INSS", "I N S S", "CONTRIB PREVIDENCIARIA", "CONTRIBUICAO PREVIDENCIARIA",
		"PREVIDENCIA SOCIAL", "PREV SOCIAL"}},
	{false, holerite.TipoIrrf, []string{"IRRF", "I R R F", "IRF", "IR", "I R", "IMPOSTO DE RENDA", "IMPOSTO RENDA",
		"IMP RENDA", "IMP DE RENDA", "IR FONTE"}},
	{false, holerite.OutroDesconto, []string{"13", "13º", "13O", "DECIMO TERCEIRO", "FERIAS", "FALTA", "FALTAS", "ATRASO",
		"ATRASOS", "PREVIDENCIA PRIVADA", "EMPRESTIMO", "CONSIGNADO"}},
	{false, holerite.ValeTransporte, []string{"VALE TRANSPORTE", "VT", "V T", "VALE TRANSP", "VALE TRANS", "TRANSPORTE"}},
	{false, holerite.ValeAlimentacao, []string{"VALE REFEICAO", "VALE ALIMENTACAO", "VR", "VA", "V R", "V A", "TICKET",
		"REFEICAO", "ALIMENTACAO", "CESTA BASICA"}},
	{false, holerite.PlanoDeSaude, []string{"PLANO DE SAUDE", "PLANO SAUDE", "ASSIST MEDICA", "ASSISTENCIA MEDICA",
		"CONVENIO MEDICO", "PLANO MEDICO", "PLANO ODONTOLOGICO", "ASSIST ODONTOLOGICA", "ASSISTENCIA ODONTOLOGICA",
		"ODONTO", "ODONTOLOGICO", "SEGURO SAUDE", "UNIMED", "COPARTICIPACAO"}},
	{false, holerite.PensaoAlimenticia, []string{"PENSAO ALIMENTICIA", "PENSAO JUDICIAL", "PENSAO"}},
	{false, holerite.TipoAdiantamento, []string{"ADIANTAMENTO", "ADIANTAMENTO SALARIAL", "ADTO", "ADIANT", "ADTO SALARIAL"}},
}

// Classificar diz o tipo da rubrica pelo dicionário, do lado em que o valor estava. Desconhecida, é "outro provento"
// ou "outro desconto", e a pessoa escolhe (false).
func Classificar(descricao string, provento bool) (int64, bool) {
	n := texto.ParaPalavras(descricao)
	for _, r := range dicionario {
		if r.provento == provento && texto.TemPalavra(n, r.termos...) {
			return r.tipo, true
		}
	}
	if provento {
		return holerite.OutroProvento, false
	}
	return holerite.OutroDesconto, false
}

// tiposDaLinha são os tipos que uma linha do holerite lido pode ter: até 9, proventos; de 10 em diante, descontos.
var tiposDaLinha = map[int64]bool{
	holerite.TipoSalario: true, holerite.HoraExtra: true, holerite.AdicionalNoturno: true, holerite.OutroProvento: true,
	holerite.TipoDSR: true, holerite.ValeTransporte: false, holerite.ValeAlimentacao: false, holerite.PlanoDeSaude: false,
	holerite.PensaoAlimenticia: false, holerite.OutroDesconto: false, holerite.TipoInss: false, holerite.TipoIrrf: false,
	holerite.TipoAdiantamento: false,
}

// ehProvento diz o lado do tipo (true, provento) e se o tipo existe.
func ehProvento(tipo int64) (provento, existe bool) {
	provento, existe = tiposDaLinha[tipo]
	return provento, existe
}
