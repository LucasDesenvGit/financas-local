package extrato

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/transacoes"
)

type contaDaTransferencia struct {
	id     int64
	nome   string
	manual bool
}

// transferir é o POST /api/transacoes/transferencia: a transferência entre contas, lançada só nas contas manuais (o
// lado do Open Finance chega pela sincronização). Os dois lados têm o mesmo grupo, a categoria Transferências
// (escolhida), e ficam fora dos relatórios. Tudo numa transação do banco (no C#, um SaveChanges só).
func (s *Servico) transferir(w http.ResponseWriter, r *http.Request) {
	origemId := respostas.CampoInteiro{Propriedade: "ContaOrigemId"}
	destinoId := respostas.CampoInteiro{Propriedade: "ContaDestinoId"}
	valor := respostas.CampoDecimal{Propriedade: "Valor", Faixa: &respostas.Faixa{Minimo: 0.01, Maximo: 999999999}}
	data := respostas.CampoData{Propriedade: "Data"}
	descricao := respostas.CampoTexto{Propriedade: "Descricao", Maximo: 300, Opcional: true}
	if !respostas.LerObjeto(w, r, "dto", "Financas.Api.Dtos.TransferenciaDto", &origemId, &destinoId, &valor, &data,
		&descricao) {
		return
	}
	if origemId.Valor == destinoId.Valor {
		respostas.Mensagem(w, http.StatusBadRequest, "Escolha contas diferentes.")
		return
	}
	contas := map[int64]contaDaTransferencia{}
	linhas, err := s.db.QueryContext(r.Context(), `SELECT "Id", "Nome", "Origem" = 1 FROM "Conta" WHERE "Id" = ? OR "Id" = ?`,
		origemId.Valor, destinoId.Valor)
	if err == nil {
		for linhas.Next() {
			var c contaDaTransferencia
			if err = linhas.Scan(&c.id, &c.nome, &c.manual); err != nil {
				break
			}
			contas[c.id] = c
		}
		linhas.Close()
		if err == nil {
			err = linhas.Err()
		}
	}
	if err != nil {
		s.erroInterno(w, r, "ler as contas da transferência", err)
		return
	}
	if len(contas) != 2 {
		respostas.Mensagem(w, http.StatusBadRequest, "Conta inválida.")
		return
	}
	origem, destino := contas[origemId.Valor], contas[destinoId.Valor]
	if !origem.manual && !destino.manual {
		respostas.Mensagem(w, http.StatusBadRequest, "Entre contas do Open Finance as transferências são importadas automaticamente.")
		return
	}
	texto := "Transferência"
	if descricao.Valor != nil && strings.TrimSpace(*descricao.Valor) != "" {
		texto = strings.TrimSpace(*descricao.Valor)
	}
	err = banco.EmTransacao(r.Context(), s.db, func(tx *sql.Tx) error {
		var categoria *int64
		err := tx.QueryRowContext(r.Context(), `SELECT "Id" FROM "Categoria" WHERE "Tipo" = ? AND "Nome" = 'Transferências' LIMIT 1`,
			transacoes.Transferencia).Scan(&categoria)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		grupo := tipos.NovoGuid()
		lancar := func(conta contaDaTransferencia, v tipos.Decimal, complemento string) error {
			agora := tipos.NovaDataHora(s.relogio.Agora())
			_, err := tx.ExecContext(r.Context(), `INSERT INTO "Transacao" ("ContaId", "PluggyTransactionId", "Data",
				"Descricao", "DescricaoOriginal", "Estabelecimento", "Valor", "Tipo", "Status", "CategoriaId", "CategoriaManual",
				"PluggyCategoriaId", "FaturaCartaoId", "ParcelaNumero", "ParcelaTotal", "IgnorarRelatorios", "Emergencia",
				"DescricaoRevisada", "MeioPagamento", "MesmaTitularidade", "RegraDescricaoId", "DescricaoManual",
				"TransferenciaGrupo", "Observacao", "CriadoEm", "AtualizadoEm")
				VALUES (?, NULL, ?, ?, NULL, NULL, ?, ?, 1, ?, 1, NULL, NULL, NULL, NULL, 1, 0, 0, NULL, NULL, NULL, 0, ?, NULL, ?, ?)`,
				conta.id, data.Valor, texto+" "+complemento, v, transacoes.Transferencia, categoria, grupo, agora, agora)
			return err
		}
		if origem.manual {
			if err := lancar(origem, tipos.DeDecimal(valor.Valor.Decimal().Neg()), "para "+destino.nome); err != nil {
				return err
			}
		}
		if destino.manual {
			return lancar(destino, valor.Valor, "de "+origem.nome)
		}
		return nil
	})
	if err != nil {
		s.erroInterno(w, r, "lançar a transferência", err)
		return
	}
	respostas.SemCorpo(w, http.StatusNoContent)
}
