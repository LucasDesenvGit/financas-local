package tipos

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	formatoData            = "2006-01-02"
	formatoDataHoraNoBanco = "2006-01-02 15:04:05.9999999" // a fração some quando é zero, como no EF Core
	formatoDataHoraNoJSON  = "2006-01-02T15:04:05.9999999"
)

// Data é o DateOnly do C#: só ano, mês e dia, sem fuso (ex.: a data de uma transação, no fuso de Brasília).
// No banco e no JSON: "2026-10-15".
type Data struct {
	t time.Time // sempre meia-noite em UTC; o fuso não significa nada aqui
}

// NovaData cria uma data.
func NovaData(ano int, mes time.Month, dia int) Data {
	return Data{t: time.Date(ano, mes, dia, 0, 0, 0, 0, time.UTC)}
}

// DataDe tira a data de um instante, no fuso em que ele está (converta para Brasília antes, se for o caso).
func DataDe(t time.Time) Data { return NovaData(t.Date()) }

// LerData lê "2026-10-15".
func LerData(texto string) (Data, error) {
	t, err := time.Parse(formatoData, strings.TrimSpace(texto))
	if err != nil {
		return Data{}, fmt.Errorf("data inválida %q: %w", texto, err)
	}
	return Data{t: t}, nil
}

func (d Data) Ano() int           { return d.t.Year() }
func (d Data) Mes() time.Month    { return d.t.Month() }
func (d Data) Dia() int           { return d.t.Day() }
func (d Data) String() string     { return d.t.Format(formatoData) }
func (d Data) Antes(o Data) bool  { return d.t.Before(o.t) }
func (d Data) Depois(o Data) bool { return d.t.After(o.t) }

// Igual compara as datas (use no lugar do ==, que compara o time.Time de dentro).
func (d Data) Igual(o Data) bool { return d.t.Equal(o.t) }

// SomarDias devolve a data n dias depois (ou antes, com n negativo).
func (d Data) SomarDias(n int) Data { return Data{t: d.t.AddDate(0, 0, n)} }

// SomarAnos é o DateOnly.AddYears do C#: o dia que não existe no ano de chegada vira o último do mês (29/02/2024 +
// 1 ano = 28/02/2025). O AddDate do Go passaria para 01/03.
func (d Data) SomarAnos(n int) Data {
	ano, mes := d.Ano()+n, d.Mes()
	ultimo := time.Date(ano, mes+1, 0, 0, 0, 0, 0, time.UTC).Day()
	return NovaData(ano, mes, min(d.Dia(), ultimo))
}

// SomarMeses é o DateOnly.AddMonths do C#: o dia que não existe no mês de chegada vira o último dia dele (31/01 + 1
// mês = 28/02, ou 29/02 em ano bissexto; 31/03 - 1 mês = 28/02). O AddDate do Go passaria para o mês seguinte
// (31/01 + 1 mês = 03/03). Cada soma parte da data recebida: somar um mês duas vezes a 31/01 dá 28/03, e não 31/03.
// O C# lança exceção quando o resultado sai do ano 1 a 9999; aqui o resultado não é conferido.
// O DateOnly.AddMonths chama o DateTime.AddMonths, que descreve a regra do último dia válido do mês:
// https://learn.microsoft.com/dotnet/api/system.datetime.addmonths (Remarks) e
// https://github.com/dotnet/runtime/blob/main/src/libraries/System.Private.CoreLib/src/System/DateOnly.cs
func (d Data) SomarMeses(n int) Data {
	// O time.Date normaliza o mês fora de 1 a 12 (inclusive o negativo): dia 1 do mês de chegada, sem estourar.
	primeiro := time.Date(d.Ano(), d.Mes()+time.Month(n), 1, 0, 0, 0, 0, time.UTC)
	ultimo := time.Date(primeiro.Year(), primeiro.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
	return NovaData(primeiro.Year(), primeiro.Month(), min(d.Dia(), ultimo))
}

// diaNumeroDe1970 é o DateOnly.DayNumber de 01/01/1970: os dias desde 01/01/0001, que é o dia 0.
const diaNumeroDe1970 = 719162

// NumeroDoDia é o DateOnly.DayNumber do C#: quantos dias passaram desde 01/01/0001 (o dia 0). A diferença entre dois
// números é o número de dias entre as datas (a conta "dias para vencer").
func (d Data) NumeroDoDia() int { return int(d.t.Unix()/86400) + diaNumeroDe1970 }

// Value grava "2026-10-15" (driver.Valuer).
func (d Data) Value() (driver.Value, error) { return d.String(), nil }

// Scan lê do banco (sql.Scanner).
func (d *Data) Scan(origem any) error {
	switch v := origem.(type) {
	case string:
		return d.lerTexto(v)
	case []byte:
		return d.lerTexto(string(v))
	case time.Time:
		*d = DataDe(v)
		return nil
	default:
		return fmt.Errorf("data gravada como %T, esperado TEXT", origem)
	}
}

func (d *Data) lerTexto(texto string) error {
	lida, err := LerData(texto)
	if err != nil {
		return err
	}
	*d = lida
	return nil
}

func (d Data) MarshalJSON() ([]byte, error) { return json.Marshal(d.String()) }

func (d *Data) UnmarshalJSON(dados []byte) error {
	var texto string
	if err := json.Unmarshal(dados, &texto); err != nil {
		return fmt.Errorf("data deve vir como texto: %w", err)
	}
	return d.lerTexto(texto)
}

// DataHora é o DateTime do C# como o EF Core guarda no SQLite: a hora "de parede", sem fuso.
//
// Quem grava decide o que ela significa: carimbos (CriadoEm, AtualizadoEm) são UTC; horários de negócio são de
// Brasília. Converta antes de criar (pacote relogio). A precisão é a do C#: 100 ns (7 casas).
//
// No banco: "2026-10-15 22:55:31.3481471". No JSON: "2026-10-15T22:55:31.3481471", sem "Z", porque o EF Core lê
// o valor de volta sem fuso (DateTimeKind.Unspecified).
type DataHora struct {
	t time.Time // sempre em UTC, guardando os números da hora de parede
}

// NovaDataHora guarda a hora de parede de t (ignora o fuso dele) com a precisão do C#.
func NovaDataHora(t time.Time) DataHora {
	parede := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.UTC)
	return DataHora{t: parede.Truncate(100 * time.Nanosecond)}
}

// LerDataHora lê "2026-10-15 22:55:31.3481471" ou "2026-10-15T22:55:31.3481471" (com "Z" opcional).
func LerDataHora(texto string) (DataHora, error) {
	texto = strings.TrimSuffix(strings.TrimSpace(texto), "Z")
	for _, formato := range []string{formatoDataHoraNoBanco, formatoDataHoraNoJSON} {
		if t, err := time.Parse(formato, texto); err == nil {
			return NovaDataHora(t), nil
		}
	}
	return DataHora{}, fmt.Errorf("data e hora inválida %q (esperado 2006-01-02 15:04:05.9999999, sem fuso)", texto)
}

// Tempo devolve a hora de parede como time.Time em UTC (o fuso não vem do banco: quem lê sabe o que ela é).
func (d DataHora) Tempo() time.Time { return d.t }
func (d DataHora) String() string   { return d.t.Format(formatoDataHoraNoBanco) }

// Value grava no formato do EF Core (driver.Valuer).
func (d DataHora) Value() (driver.Value, error) { return d.String(), nil }

// Scan lê do banco (sql.Scanner).
func (d *DataHora) Scan(origem any) error {
	switch v := origem.(type) {
	case string:
		return d.lerTexto(v)
	case []byte:
		return d.lerTexto(string(v))
	case time.Time:
		*d = NovaDataHora(v)
		return nil
	default:
		return fmt.Errorf("data e hora gravada como %T, esperado TEXT", origem)
	}
}

func (d *DataHora) lerTexto(texto string) error {
	lida, err := LerDataHora(texto)
	if err != nil {
		return err
	}
	*d = lida
	return nil
}

func (d DataHora) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.t.Format(formatoDataHoraNoJSON))
}

func (d *DataHora) UnmarshalJSON(dados []byte) error {
	var texto string
	if err := json.Unmarshal(dados, &texto); err != nil {
		return fmt.Errorf("data e hora deve vir como texto: %w", err)
	}
	return d.lerTexto(texto)
}
