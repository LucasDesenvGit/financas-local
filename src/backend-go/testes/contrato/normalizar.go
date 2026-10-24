package contrato

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Normalizador troca o que muda a cada execução por marcadores, e só isso. O que não muda (ids do AUTOINCREMENT,
// dinheiro, datas de negócio, mensagens) fica cru, porque o Go tem de produzir exatamente o mesmo.
//
// O que é trocado, e por quê (detalhes no README.md):
//   - data e hora que saiu do relógio do processo: o servidor sobe com o relógio em Inicio (faketime no .NET), mas o
//     relógio anda, e cada execução cai em outra fração de segundo. Vira "<relógio +N min, formato>", com N os
//     minutos desde Inicio: um horário em outro fuso (3 h antes) fica fora da janela e aparece cru na diferença;
//   - GUID gerado na hora (carimbo de segurança): "<GUID>" ou "<guid>", guardando se veio em maiúsculas;
//   - traceId do ValidationProblemDetails: "<traceId>";
//   - a versão do sistema (sobe a cada PR, ADR 0039): "<versao>";
//   - a pasta temporária dos dados: "<dados>", com o resto do caminho com "/" (no Windows o .NET e o Go usam "\\", e o
//     esperado gravado no Linux vale nos dois);
//   - o hash da senha (sal aleatório): "<hash v3 ...>" com os parâmetros, que têm de ser os mesmos;
//   - o valor dos cookies (cifrado pelo Data Protection): "<valor>".
type Normalizador struct {
	Inicio     time.Time // instante em que o relógio do servidor começou
	Versao     string    // a do Directory.Build.props
	PastaDados string    // pasta com sql/ e chaves/ do servidor sob teste
}

// Janela em que um horário é considerado "agora": do início até um dia depois.
const janelaRelogio = 24 * time.Hour

var (
	// Formato do EF Core no SQLite: "2026-10-15 22:55:31.3481471" (fração de até 7 casas, sem zeros no fim).
	dataHoraEf = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}) (\d{2}:\d{2}:\d{2})(\.\d{1,7})?$`)
	// ISO 8601 do System.Text.Json e do ToString("O"): "2026-10-15T22:55:31.3481471", com Z ou deslocamento.
	dataHoraIso = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})T(\d{2}:\d{2}:\d{2})(\.\d{1,7})?(Z|[+-]\d{2}:\d{2})?$`)
	guidMaiusc  = regexp.MustCompile(`^[0-9A-F]{8}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{12}$`)
	guidMinusc  = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	traceID     = regexp.MustCompile(`^00-[0-9a-f]{32}-[0-9a-f]{16}-[0-9a-f]{2}$`)
)

// instante lê uma data e hora num dos dois formatos. Sem fuso, é UTC (o EF Core grava UTC sem marcador).
func instante(s string) (t time.Time, formato string, ok bool) {
	if m := dataHoraEf.FindStringSubmatch(s); m != nil {
		t, err := time.Parse("2006-01-02 15:04:05.9999999", m[1]+" "+m[2]+m[3])
		return t, "AAAA-MM-DD hh:mm:ss.f", err == nil
	}
	if m := dataHoraIso.FindStringSubmatch(s); m != nil {
		zona := m[4]
		texto := m[1] + "T" + m[2] + m[3]
		var err error
		switch zona {
		case "":
			t, err = time.Parse("2006-01-02T15:04:05.9999999", texto)
		case "Z":
			t, err = time.Parse("2006-01-02T15:04:05.9999999Z", texto+"Z")
		default:
			t, err = time.Parse("2006-01-02T15:04:05.9999999-07:00", texto+zona)
		}
		return t, "AAAA-MM-DDThh:mm:ss.f" + zona, err == nil
	}
	return time.Time{}, "", false
}

// texto normaliza um texto solto (da resposta ou de uma coluna TEXT).
func (n Normalizador) texto(s string) string {
	if t, formato, ok := instante(s); ok {
		if d := t.Sub(n.Inicio); d >= 0 && d < janelaRelogio {
			return fmt.Sprintf("<relógio +%d min, %s>", int(d/time.Minute), formato)
		}
		return s
	}
	switch {
	case guidMaiusc.MatchString(s):
		return "<GUID>"
	case guidMinusc.MatchString(s):
		return "<guid>"
	case n.PastaDados != "" && strings.HasPrefix(s, n.PastaDados):
		return "<dados>" + strings.ReplaceAll(strings.TrimPrefix(s, n.PastaDados), `\`, "/")
	}
	return s
}

// corpo normaliza o corpo de uma resposta: primeiro os campos que o caso pede, depois as regras gerais.
func (n Normalizador) corpo(v any, pedidos map[string]string) (any, error) {
	ponteiros := make([]string, 0, len(pedidos))
	for p := range pedidos {
		ponteiros = append(ponteiros, p)
	}
	sort.Strings(ponteiros)
	for _, p := range ponteiros {
		valor, trocar, err := ponteiro(v, p)
		if err != nil {
			return nil, fmt.Errorf("normalizar: %w", err)
		}
		marcador, err := marcadorPedido(pedidos[p], valor)
		if err != nil {
			return nil, fmt.Errorf("normalizar %s: %w", p, err)
		}
		trocar(marcador)
	}
	return n.geral("", v), nil
}

func marcadorPedido(tipo string, valor any) (string, error) {
	switch tipo {
	case "texto":
		if s, ok := valor.(string); ok && s != "" {
			return "<texto>", nil
		}
	case "numero":
		if _, ok := valor.(json.Number); ok {
			return "<número>", nil
		}
	case "datahora":
		if s, ok := valor.(string); ok {
			if _, formato, ok := instante(s); ok {
				return "<data e hora, " + formato + ">", nil
			}
		}
	}
	return "", fmt.Errorf("esperado %s, veio %s", tipo, mostrar(valor))
}

func (n Normalizador) geral(chave string, v any) any {
	switch x := v.(type) {
	case map[string]any:
		for k, sub := range x {
			x[k] = n.geral(k, sub)
		}
		return x
	case []any:
		for i, sub := range x {
			x[i] = n.geral(chave, sub)
		}
		return x
	case string:
		switch {
		case chave == "traceId" && traceID.MatchString(x):
			return "<traceId>"
		case chave == "versao" && n.Versao != "" && x == n.Versao:
			return "<versao>"
		}
		return n.texto(x)
	}
	return v
}

// colunaTexto normaliza o valor de uma coluna TEXT do banco.
func (n Normalizador) colunaTexto(tabela, coluna, valor string) string {
	if tabela == "Usuario" && coluna == "SenhaHash" {
		return hashSenha(valor)
	}
	return n.texto(valor)
}

// hashSenha descreve um hash do PasswordHasher do ASP.NET Identity, formato v3 (0x01, PRF, iterações, tamanho do
// sal, sal, chave; inteiros em big-endian). O sal é aleatório; o resto tem de ser igual para o .NET ler a senha que
// o Go gravou. https://github.com/dotnet/aspnetcore/blob/main/src/Identity/Extensions.Core/src/PasswordHasher.cs
func hashSenha(valor string) string {
	b, err := base64.StdEncoding.DecodeString(valor)
	if err != nil || len(b) < 13 || b[0] != 0x01 {
		return valor // fora do formato v3: fica cru e aparece na diferença
	}
	prf := binary.BigEndian.Uint32(b[1:5])
	iteracoes := binary.BigEndian.Uint32(b[5:9])
	sal := int(binary.BigEndian.Uint32(b[9:13]))
	if 13+sal > len(b) {
		return valor
	}
	return fmt.Sprintf("<hash v3: prf %d, %d iterações, sal de %d bytes, chave de %d bytes>", prf, iteracoes, sal, len(b)-13-sal)
}

// cookie normaliza um Set-Cookie: troca o valor por "<valor>" (vazio continua vazio, é o cookie sendo apagado) e
// põe os atributos em minúsculas e em ordem, porque o navegador não liga para a ordem nem para maiúsculas.
func cookie(setCookie string) string {
	partes := strings.Split(setCookie, ";")
	nome, valor, _ := strings.Cut(strings.TrimSpace(partes[0]), "=")
	if valor != "" {
		valor = "<valor>"
	}
	atributos := make([]string, 0, len(partes)-1)
	for _, a := range partes[1:] {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		k, v, temValor := strings.Cut(a, "=")
		k = strings.ToLower(k)
		if temValor {
			if k == "samesite" {
				v = strings.ToLower(v)
			}
			atributos = append(atributos, k+"="+v)
		} else {
			atributos = append(atributos, k)
		}
	}
	sort.Strings(atributos)
	return strings.Join(append([]string{nome + "=" + valor}, atributos...), "; ")
}
