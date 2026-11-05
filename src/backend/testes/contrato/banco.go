package contrato

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strings"

	_ "modernc.org/sqlite" // driver "sqlite", Go puro
)

// TabelasIgnoradas ficam fora da comparação do banco: são gravadas por tarefas em segundo plano do .NET, em
// momentos que não dependem dos casos. ErroSistema recebe os avisos do log (ADR 0022), como o do câmbio e o da
// cripto, que tentam a internet 1 minuto depois de subir e falham, porque o servidor roda sem rede.
var TabelasIgnoradas = []string{"ErroSistema"}

// Foto é o conteúdo do banco num momento: tabela → linhas em ordem de chave primária.
type Foto map[string]*fotoTabela

type fotoTabela struct {
	chave  []string // colunas da chave primária (ou rowid)
	ordem  []string // chaves das linhas, em ordem
	linhas map[string]map[string]any
}

// DiferencaTabela é o que um caso mudou numa tabela. As linhas vêm com todas as colunas, normalizadas; nas
// alteradas, só as colunas que mudaram (comparadas pelo valor cru, antes de normalizar).
type DiferencaTabela struct {
	Inseridas []map[string]any `json:"inseridas,omitempty"`
	Alteradas []LinhaAlterada  `json:"alteradas,omitempty"`
	Removidas []map[string]any `json:"removidas,omitempty"`
}

// LinhaAlterada é uma linha que mudou: a chave primária e, por coluna, o antes e o depois.
type LinhaAlterada struct {
	Chave   map[string]any          `json:"chave"`
	Colunas map[string]AntesEDepois `json:"colunas"`
}

// AntesEDepois é o valor de uma coluna antes e depois do caso.
type AntesEDepois struct {
	Antes  any `json:"antes"`
	Depois any `json:"depois"`
}

// valorCru é o valor de uma coluna como o SQLite guarda: o tipo de armazenamento e o texto.
type valorCru struct {
	tipo  string // integer, real, text, blob, null
	texto string
}

// abrirSoLeitura abre o banco do servidor sob teste só para ler.
func abrirSoLeitura(caminho string) (*sql.DB, error) {
	p := url.Values{}
	p.Set("mode", "ro")
	p.Add("_pragma", "busy_timeout(30000)")
	db, err := sql.Open("sqlite", "file:"+caminho+"?"+p.Encode())
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("abrir %s: %w", caminho, err)
	}
	return db, nil
}

// fotografar lê todas as tabelas (menos as ignoradas), com o tipo e o texto cru de cada coluna, numa transação só
// de leitura (todas as tabelas no mesmo instante).
func fotografar(ctx context.Context, db *sql.DB) (Foto, error) {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var tabelas []string
	linhas, err := tx.QueryContext(ctx, `SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		return nil, err
	}
	for linhas.Next() {
		var nome string
		if err := linhas.Scan(&nome); err != nil {
			linhas.Close()
			return nil, err
		}
		if !slices.Contains(TabelasIgnoradas, nome) {
			tabelas = append(tabelas, nome)
		}
	}
	linhas.Close()
	if err := linhas.Err(); err != nil {
		return nil, err
	}

	foto := Foto{}
	for _, tabela := range tabelas {
		ft, err := fotografarTabela(ctx, tx, tabela)
		if err != nil {
			return nil, fmt.Errorf("ler %s: %w", tabela, err)
		}
		foto[tabela] = ft
	}
	return foto, nil
}

func aspas(nome string) string { return `"` + strings.ReplaceAll(nome, `"`, `""`) + `"` }

func fotografarTabela(ctx context.Context, tx *sql.Tx, tabela string) (*fotoTabela, error) {
	// Colunas e chave primária. O nome da tabela vem do sqlite_master, não de fora.
	info, err := tx.QueryContext(ctx, `SELECT name, pk FROM pragma_table_info(?) ORDER BY cid`, tabela)
	if err != nil {
		return nil, err
	}
	var colunas []string
	chavePorPosicao := map[int]string{}
	for info.Next() {
		var nome string
		var pk int
		if err := info.Scan(&nome, &pk); err != nil {
			info.Close()
			return nil, err
		}
		colunas = append(colunas, nome)
		if pk > 0 {
			chavePorPosicao[pk] = nome
		}
	}
	info.Close()
	chave := make([]string, 0, len(chavePorPosicao))
	for i := 1; i <= len(chavePorPosicao); i++ {
		chave = append(chave, chavePorPosicao[i])
	}
	selecionar := make([]string, 0, 2*len(colunas)+1)
	for _, c := range colunas {
		// hex() para BLOB; CAST AS TEXT para o resto (INTEGER e REAL no texto do próprio SQLite).
		selecionar = append(selecionar, "typeof("+aspas(c)+")",
			"CASE typeof("+aspas(c)+") WHEN 'blob' THEN hex("+aspas(c)+") ELSE CAST("+aspas(c)+" AS TEXT) END")
	}
	ordenar := "rowid"
	if len(chave) > 0 {
		partes := make([]string, len(chave))
		for i, c := range chave {
			partes[i] = aspas(c)
		}
		ordenar = strings.Join(partes, ", ")
	} else {
		selecionar = append(selecionar, "rowid")
		chave = []string{"rowid"}
	}
	consulta := "SELECT " + strings.Join(selecionar, ", ") + " FROM " + aspas(tabela) + " ORDER BY " + ordenar
	linhas, err := tx.QueryContext(ctx, consulta)
	if err != nil {
		return nil, err
	}
	defer linhas.Close()

	ft := &fotoTabela{chave: chave, linhas: map[string]map[string]any{}}
	for linhas.Next() {
		destinos := make([]any, len(selecionar))
		brutos := make([]sql.NullString, len(selecionar))
		for i := range destinos {
			destinos[i] = &brutos[i]
		}
		if err := linhas.Scan(destinos...); err != nil {
			return nil, err
		}
		linha := map[string]any{}
		for i, c := range colunas {
			linha[c] = valorCru{tipo: brutos[2*i].String, texto: brutos[2*i+1].String}
		}
		if chave[0] == "rowid" && len(colunas)*2 < len(brutos) {
			linha["rowid"] = valorCru{tipo: "integer", texto: brutos[len(brutos)-1].String}
		}
		id := make([]string, len(chave))
		for i, c := range chave {
			id[i] = linha[c].(valorCru).texto
		}
		k := strings.Join(id, "\x00")
		ft.ordem = append(ft.ordem, k)
		ft.linhas[k] = linha
	}
	return ft, linhas.Err()
}

// valorJSON é como uma coluna aparece no esperado: INTEGER vira número, TEXT vira texto (normalizado), NULL vira
// null. REAL e BLOB levam o tipo no texto ("real:1.5", "blob:0A0B"): no formato do EF Core, decimal e data são
// TEXT, e um REAL no lugar deles é diferença que tem de aparecer.
func (n Normalizador) valorJSON(tabela, coluna string, v valorCru) any {
	switch v.tipo {
	case "null":
		return nil
	case "integer":
		return json.Number(v.texto)
	case "text":
		return n.colunaTexto(tabela, coluna, v.texto)
	}
	return v.tipo + ":" + v.texto
}

func (n Normalizador) linhaJSON(tabela string, linha map[string]any) map[string]any {
	saida := make(map[string]any, len(linha))
	for c, v := range linha {
		saida[c] = n.valorJSON(tabela, c, v.(valorCru))
	}
	return saida
}

// diferencas compara duas fotos e devolve o que mudou, por tabela.
func (n Normalizador) diferencas(antes, depois Foto) map[string]*DiferencaTabela {
	nomes := map[string]bool{}
	for t := range antes {
		nomes[t] = true
	}
	for t := range depois {
		nomes[t] = true
	}
	saida := map[string]*DiferencaTabela{}
	for tabela := range nomes {
		a, d := antes[tabela], depois[tabela]
		if a == nil {
			a = &fotoTabela{linhas: map[string]map[string]any{}}
		}
		if d == nil {
			d = &fotoTabela{linhas: map[string]map[string]any{}}
		}
		dif := &DiferencaTabela{}
		for _, k := range d.ordem {
			la, existia := a.linhas[k]
			ld := d.linhas[k]
			if !existia {
				dif.Inseridas = append(dif.Inseridas, n.linhaJSON(tabela, ld))
				continue
			}
			colunas := map[string]AntesEDepois{}
			for c, vd := range ld {
				if va, ok := la[c]; !ok || va != vd {
					var antesJSON any
					if ok {
						antesJSON = n.valorJSON(tabela, c, va.(valorCru))
					}
					colunas[c] = AntesEDepois{Antes: antesJSON, Depois: n.valorJSON(tabela, c, vd.(valorCru))}
				}
			}
			if len(colunas) > 0 {
				chave := map[string]any{}
				for _, c := range d.chave {
					chave[c] = n.valorJSON(tabela, c, ld[c].(valorCru))
				}
				dif.Alteradas = append(dif.Alteradas, LinhaAlterada{Chave: chave, Colunas: colunas})
			}
		}
		for _, k := range a.ordem {
			if _, ficou := d.linhas[k]; !ficou {
				dif.Removidas = append(dif.Removidas, n.linhaJSON(tabela, a.linhas[k]))
			}
		}
		if len(dif.Inseridas)+len(dif.Alteradas)+len(dif.Removidas) > 0 {
			saida[tabela] = dif
		}
	}
	return saida
}
