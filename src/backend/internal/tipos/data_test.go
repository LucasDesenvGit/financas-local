package tipos

import (
	"testing"
	"time"
)

// Regra: SomarMeses é o DateOnly.AddMonths do C# (#447): o dia que não existe no mês de chegada vira o último dele, e
// o AddDate do Go passaria para o mês seguinte. A regra e os exemplos (31/03 + 1 mês = 30/04; 31/03 - 1 mês = 28/02 ou
// 29/02) são os da documentação do DateTime.AddMonths, que o DateOnly.AddMonths chama.
func TestSomarMeses(t *testing.T) {
	d := NovaData
	casos := []struct {
		nome      string
		de        Data
		meses     int
		esperado  Data
		doAddDate Data // o que o time.AddDate(0, n, 0) daria: o que a função existe para evitar
	}{
		{"dia 31 em mês de 28 dias", d(2026, 1, 31), 1, d(2026, 2, 28), d(2026, 3, 3)},
		{"dia 31 em fevereiro bissexto", d(2024, 1, 31), 1, d(2024, 2, 29), d(2024, 3, 2)},
		{"dia 30 em fevereiro", d(2026, 1, 30), 1, d(2026, 2, 28), d(2026, 3, 2)},
		{"dia 29 em fevereiro de ano comum", d(2026, 1, 29), 1, d(2026, 2, 28), d(2026, 3, 1)},
		{"dia 29 em fevereiro bissexto cabe", d(2024, 1, 29), 1, d(2024, 2, 29), d(2024, 2, 29)},
		{"dia 31 em mês de 30 dias", d(2026, 3, 31), 1, d(2026, 4, 30), d(2026, 5, 1)},
		{"dia 31 em mês de 31 dias", d(2026, 7, 31), 1, d(2026, 8, 31), d(2026, 8, 31)},
		{"dia 30 em mês de 30 dias", d(2026, 4, 30), 1, d(2026, 5, 30), d(2026, 5, 30)},
		{"dia comum", d(2026, 10, 15), 1, d(2026, 11, 15), d(2026, 11, 15)},
		{"virada de ano", d(2025, 12, 15), 1, d(2026, 1, 15), d(2026, 1, 15)},
		{"virada de ano com dia que some", d(2025, 12, 31), 2, d(2026, 2, 28), d(2026, 3, 3)},
		{"zero meses", d(2026, 1, 31), 0, d(2026, 1, 31), d(2026, 1, 31)},
		{"doze meses", d(2026, 5, 31), 12, d(2027, 5, 31), d(2027, 5, 31)},
		{"29/02 mais doze meses", d(2024, 2, 29), 12, d(2025, 2, 28), d(2025, 3, 1)},
		{"vários anos", d(2026, 8, 31), 30, d(2029, 2, 28), d(2029, 3, 3)},
		{"voltar um mês", d(2026, 3, 31), -1, d(2026, 2, 28), d(2026, 3, 3)},
		{"voltar na virada de ano", d(2026, 1, 1), -1, d(2025, 12, 1), d(2025, 12, 1)},
		{"voltar o dia 31 para mês de 30", d(2026, 12, 31), -1, d(2026, 11, 30), d(2026, 12, 1)},
		{"voltar seis meses (janela do histórico)", d(2026, 10, 1), -6, d(2026, 4, 1), d(2026, 4, 1)},
		{"voltar mais de um ano", d(2026, 3, 31), -25, d(2024, 2, 29), d(2024, 3, 2)},
		{"voltar doze meses de 29/02", d(2024, 2, 29), -12, d(2023, 2, 28), d(2023, 3, 1)},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if obtido := c.de.SomarMeses(c.meses); !obtido.Igual(c.esperado) {
				t.Errorf("%s %+d meses = %s, esperado %s", c.de, c.meses, obtido, c.esperado)
			}
			// A diferença para o AddDate é o motivo da função: quando o dia some, o AddDate vai para o mês seguinte.
			addDate := time.Date(c.de.Ano(), c.de.Mes(), c.de.Dia(), 0, 0, 0, 0, time.UTC).AddDate(0, c.meses, 0)
			if !DataDe(addDate).Igual(c.doAddDate) {
				t.Errorf("o AddDate deu %s, a tabela diz %s: a coluna doAddDate está errada", DataDe(addDate), c.doAddDate)
			}
		})
	}
}

// Somar um mês parte da data que recebeu: 31/01 + 1 mês + 1 mês é 28/03 (e não 31/03), como o C# faz ao avançar o
// vencimento de uma conta a pagar a cada pagamento.
func TestSomarMesesEmCadeiaNaoVoltaAoDia31(t *testing.T) {
	d := NovaData(2026, 1, 31).SomarMeses(1).SomarMeses(1)
	if esperado := NovaData(2026, 3, 28); !d.Igual(esperado) {
		t.Errorf("31/01 + 1 mês + 1 mês = %s, esperado %s", d, esperado)
	}
}

// Regra: NumeroDoDia é o DateOnly.DayNumber do C#: dias desde 01/01/0001 (o dia 0). Os valores de âncora vêm do
// ordinal do Python (date.toordinal() - 1), uma conta independente da do código.
func TestNumeroDoDia(t *testing.T) {
	casos := []struct {
		data     Data
		esperado int
	}{
		{NovaData(1, 1, 1), 0},
		{NovaData(1969, 12, 31), 719161},
		{NovaData(1970, 1, 1), 719162},
		{NovaData(2000, 1, 1), 730119},
		{NovaData(2024, 2, 29), 738944},
		{NovaData(2026, 10, 29), 739917},
		{NovaData(9999, 12, 31), 3652058},
	}
	for _, c := range casos {
		t.Run(c.data.String(), func(t *testing.T) {
			if obtido := c.data.NumeroDoDia(); obtido != c.esperado {
				t.Errorf("NumeroDoDia(%s) = %d, esperado %d", c.data, obtido, c.esperado)
			}
		})
	}
}

// A diferença entre dois números é o número de dias entre as datas: fevereiro bissexto, virada de ano e o sinal.
func TestNumeroDoDiaDiferenca(t *testing.T) {
	casos := []struct {
		nome     string
		de, ate  Data
		esperado int
	}{
		{"mesmo dia", NovaData(2026, 10, 22), NovaData(2026, 10, 22), 0},
		{"dia seguinte", NovaData(2026, 10, 22), NovaData(2026, 10, 23), 1},
		{"atrasada", NovaData(2026, 10, 22), NovaData(2026, 10, 20), -2},
		{"fevereiro bissexto", NovaData(2024, 2, 28), NovaData(2024, 3, 1), 2},
		{"fevereiro comum", NovaData(2026, 2, 28), NovaData(2026, 3, 1), 1},
		{"virada de ano", NovaData(2025, 12, 31), NovaData(2026, 1, 1), 1},
		{"um ano comum", NovaData(2026, 1, 1), NovaData(2027, 1, 1), 365},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if obtido := c.ate.NumeroDoDia() - c.de.NumeroDoDia(); obtido != c.esperado {
				t.Errorf("%s até %s = %d dias, esperado %d", c.de, c.ate, obtido, c.esperado)
			}
		})
	}
}
