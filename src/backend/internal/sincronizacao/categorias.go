package sincronizacao

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/pluggy"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/texto"
)

const tamanhoDoIdDeCategoria = 20

// atualizarCategorias traz a lista de categorias da Pluggy. Categoria nova ganha a sua que combinar; as que já existem
// mudam só o nome e o pai, e o mapa escolhido à mão fica.
func (s *Servico) atualizarCategorias(ctx context.Context, sessao pluggy.Sessao) error {
	lista, err := s.pluggy.ListarCategorias(ctx, sessao)
	if err != nil || len(lista) == 0 {
		return err
	}
	porId := map[string]*pluggy.Categoria{}
	var ordem []*pluggy.Categoria
	for i := range lista {
		c := &lista[i]
		if strings.TrimSpace(c.Id) == "" || porId[c.Id] != nil {
			continue
		}
		porId[c.Id] = c
		ordem = append(ordem, c)
	}
	return banco.EmTransacao(ctx, s.db, func(tx *sql.Tx) error {
		existentes, err := lerIds(ctx, tx)
		if err != nil {
			return err
		}
		idPorNome, err := categoriasPorNome(ctx, tx)
		if err != nil {
			return err
		}
		for _, c := range ordem {
			if texto.TamanhoUtf16(c.Id) > tamanhoDoIdDeCategoria {
				continue
			}
			if err := gravarCategoria(ctx, tx, c, porId, existentes, idPorNome); err != nil {
				return err
			}
		}
		return nil
	})
}

func gravarCategoria(ctx context.Context, tx *sql.Tx, c *pluggy.Categoria, porId map[string]*pluggy.Categoria, existentes map[string]bool, idPorNome map[string]int64) error {
	nome := primeiroDe(c.DescriptionTranslated, c.Description, &c.Id)
	var pai *string
	if c.ParentId != nil {
		if p := porId[*c.ParentId]; p != nil {
			pai = primeiroDe(p.DescriptionTranslated, p.Description)
		}
	}
	if pai == nil {
		pai = c.ParentDescription
	}
	descricao := limitado(*nome, 150)
	paiDescricao := limitar(pai, 150)
	var paiId *string
	if c.ParentId != nil && texto.TamanhoUtf16(*c.ParentId) <= tamanhoDoIdDeCategoria {
		paiId = c.ParentId
	}
	if existentes[c.Id] {
		_, err := tx.ExecContext(ctx, `UPDATE "CategoriaPluggy" SET "Descricao" = ?, "PaiPluggyId" = ?, "PaiDescricao" = ? WHERE "PluggyCategoriaId" = ?`, descricao, paiId, paiDescricao, c.Id)
		return err
	}
	var categoria *int64
	if sugestao := sugerirCategoriaPadrao(c.Description, c.ParentDescription); sugestao != "" {
		if id, ok := idPorNome[texto.MaiusculasInvariantes(sugestao)]; ok {
			categoria = &id
		}
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO "CategoriaPluggy" ("PluggyCategoriaId", "Descricao", "PaiPluggyId", "PaiDescricao", "CategoriaId") VALUES (?, ?, ?, ?, ?)`,
		c.Id, descricao, paiId, paiDescricao, categoria)
	return err
}

func primeiroDe(valores ...*string) *string {
	for _, v := range valores {
		if v != nil {
			return v
		}
	}
	return nil
}

func lerIds(ctx context.Context, tx *sql.Tx) (map[string]bool, error) {
	linhas, err := tx.QueryContext(ctx, `SELECT "PluggyCategoriaId" FROM "CategoriaPluggy"`)
	if err != nil {
		return nil, err
	}
	defer linhas.Close()
	ids := map[string]bool{}
	for linhas.Next() {
		var id string
		if err := linhas.Scan(&id); err != nil {
			return nil, err
		}
		ids[id] = true
	}
	return ids, linhas.Err()
}

// categoriasPorNome é o ToDictionary do C#, sem diferença de maiúsculas. Nome repetido é erro, como a exceção do C#.
func categoriasPorNome(ctx context.Context, tx *sql.Tx) (map[string]int64, error) {
	linhas, err := tx.QueryContext(ctx, `SELECT "Id", "Nome" FROM "Categoria"`)
	if err != nil {
		return nil, err
	}
	defer linhas.Close()
	ids := map[string]int64{}
	for linhas.Next() {
		var id int64
		var nome string
		if err := linhas.Scan(&id, &nome); err != nil {
			return nil, err
		}
		chave := texto.MaiusculasInvariantes(nome)
		if _, repetido := ids[chave]; repetido {
			return nil, fmt.Errorf("categoria com o nome repetido: %q", nome)
		}
		ids[chave] = id
	}
	return ids, linhas.Err()
}
