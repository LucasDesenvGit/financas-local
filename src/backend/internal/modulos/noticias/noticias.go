// Pacote noticias é o módulo Notícias (#10, ADRs 0064 e 0067): as notícias do dia do G1, da BBC News Brasil e do
// TabNews. Nasce desligado (acessa a internet). Ligado, a tarefa busca as fontes a cada 45 minutos e guarda as 20 mais
// novas de cada uma; a tela mostra o que está guardado. Uma fonte que falha guarda o erro e não derruba as outras.
package noticias

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/noticias/fontes"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/modulos"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// ID é o id do módulo: o prefixo /api/noticias/ e a chave Modulos.noticias.Ligado.
const ID = "noticias"

// Intervalos: a tarefa, e o mínimo entre dois "Atualizar agora".
const (
	Intervalo      = 45 * time.Minute
	IntervaloAMao  = 5 * time.Minute
	MensagemEspera = "As notícias foram atualizadas há pouco. Tente de novo em alguns minutos."
)

// Modulo é o Notícias.
type Modulo struct {
	db      *sql.DB
	cliente *fontes.Cliente
	fontes  []fontes.Fonte
	relogio relogio.Relogio
	log     *slog.Logger

	mu          sync.Mutex
	ultimaAMao  time.Time
	atualizando sync.Mutex // uma atualização por vez (a tarefa e o botão)
}

// Novo monta o módulo com as fontes da #10.
func Novo(db *sql.DB, r relogio.Relogio, log *slog.Logger) *Modulo {
	return NovoCom(db, fontes.NovoCliente(nil), fontes.Padrao(), r, log)
}

// NovoCom monta o módulo com o cliente e as fontes dados (os testes apontam para um servidor de mentira).
func NovoCom(db *sql.DB, cliente *fontes.Cliente, lista []fontes.Fonte, r relogio.Relogio, log *slog.Logger) *Modulo {
	return &Modulo{db: db, cliente: cliente, fontes: lista, relogio: r, log: log}
}

// Info é o que Configurações › Módulos mostra.
func (m *Modulo) Info() modulos.Info {
	return modulos.Info{
		ID:        ID,
		Nome:      "Notícias",
		Descricao: "As notícias do dia do G1, da BBC News Brasil e do TabNews: o título, a fonte e o link, que abre no site da notícia. Busca as fontes a cada 45 minutos.",
		Destinos:  fontes.Destinos(fontes.Padrao()),
	}
}

// Registrar põe as rotas do módulo, guardadas pelo núcleo.
func (m *Modulo) Registrar(r *modulos.Rotas) {
	r.HandleFunc("GET /api/noticias", m.listarHTTP)
	r.HandleFunc("POST /api/noticias/atualizar", m.atualizarHTTP)
}

// Tarefas: buscar as fontes a cada 45 minutos (só com o módulo ligado; quem confere é o núcleo).
func (m *Modulo) Tarefas() []modulos.Tarefa {
	return []modulos.Tarefa{{Nome: "as notícias", Atraso: time.Minute, Intervalo: Intervalo, Ciclo: m.Atualizar}}
}

// Atualizar busca cada fonte e troca as notícias guardadas dela. A fonte que falha guarda o erro e fica com as notícias
// de antes; o erro de todas volta junto (vira log, e aparece em Configurações › Erros).
func (m *Modulo) Atualizar(ctx context.Context) error {
	m.atualizando.Lock()
	defer m.atualizando.Unlock()
	var falhas []error
	for _, f := range m.fontes {
		agora := tipos.NovaDataHora(m.relogio.Agora())
		itens, err := m.cliente.Buscar(ctx, f)
		if err != nil {
			falhas = append(falhas, err)
			if _, errGravar := m.db.ExecContext(ctx, `INSERT INTO "NoticiaFonte" ("Fonte", "TentadaEm", "Erro") VALUES (?, ?, ?)
				ON CONFLICT ("Fonte") DO UPDATE SET "TentadaEm" = excluded."TentadaEm", "Erro" = excluded."Erro"`, f.ID, agora, err.Error()); errGravar != nil {
				return errGravar
			}
			continue
		}
		err = banco.EmTransacao(ctx, m.db, func(tx *sql.Tx) error {
			if _, err := tx.ExecContext(ctx, `DELETE FROM "Noticia" WHERE "Fonte" = ?`, f.ID); err != nil {
				return err
			}
			for _, it := range itens {
				var publicada *tipos.DataHora
				if !it.PublicadaEm.IsZero() {
					p := tipos.NovaDataHora(it.PublicadaEm)
					publicada = &p
				}
				if _, err := tx.ExecContext(ctx, `INSERT INTO "Noticia" ("Fonte", "Titulo", "Link", "PublicadaEm", "AtualizadoEm")
					VALUES (?, ?, ?, ?, ?)`, f.ID, it.Titulo, it.Link, publicada, agora); err != nil {
					return err
				}
			}
			_, err := tx.ExecContext(ctx, `INSERT INTO "NoticiaFonte" ("Fonte", "AtualizadoEm", "TentadaEm", "Erro") VALUES (?, ?, ?, NULL)
				ON CONFLICT ("Fonte") DO UPDATE SET "AtualizadoEm" = excluded."AtualizadoEm", "TentadaEm" = excluded."TentadaEm", "Erro" = NULL`,
				f.ID, agora, agora)
			return err
		})
		if err != nil {
			return fmt.Errorf("guardar as notícias de %s: %w", f.Nome, err)
		}
	}
	return errors.Join(falhas...)
}

// SituacaoDaFonte é o estado de uma fonte para a tela.
type SituacaoDaFonte struct {
	Id           string          `json:"id"`
	Nome         string          `json:"nome"`
	AtualizadoEm *tipos.DataHora `json:"atualizadoEm"` // a última vez que deu certo
	Erro         *string         `json:"erro"`         // o erro da última tentativa
}

// Noticia é uma notícia guardada.
type Noticia struct {
	Fonte       string          `json:"fonte"`
	FonteNome   string          `json:"fonteNome"`
	Titulo      string          `json:"titulo"`
	Link        string          `json:"link"`
	PublicadaEm *tipos.DataHora `json:"publicadaEm"`
}

// Lista é o que a tela mostra.
type Lista struct {
	Fontes   []SituacaoDaFonte `json:"fontes"`
	Noticias []Noticia         `json:"noticias"`
}

// Listar devolve as fontes com a situação e as notícias guardadas, das mais novas para as mais antigas.
func (m *Modulo) Listar(ctx context.Context) (Lista, error) {
	nomes := map[string]string{}
	l := Lista{Fontes: []SituacaoDaFonte{}, Noticias: []Noticia{}}
	for _, f := range m.fontes {
		nomes[f.ID] = f.Nome
		s := SituacaoDaFonte{Id: f.ID, Nome: f.Nome}
		err := m.db.QueryRowContext(ctx, `SELECT "AtualizadoEm", "Erro" FROM "NoticiaFonte" WHERE "Fonte" = ?`, f.ID).Scan(&s.AtualizadoEm, &s.Erro)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return Lista{}, fmt.Errorf("ler a fonte %s: %w", f.Nome, err)
		}
		l.Fontes = append(l.Fontes, s)
	}
	linhas, err := m.db.QueryContext(ctx, `SELECT "Fonte", "Titulo", "Link", "PublicadaEm" FROM "Noticia" ORDER BY "Id"`)
	if err != nil {
		return Lista{}, fmt.Errorf("ler as notícias: %w", err)
	}
	defer linhas.Close()
	for linhas.Next() {
		var n Noticia
		if err := linhas.Scan(&n.Fonte, &n.Titulo, &n.Link, &n.PublicadaEm); err != nil {
			return Lista{}, err
		}
		n.FonteNome = nomes[n.Fonte]
		if n.FonteNome != "" { // fonte que saiu da lista não aparece
			l.Noticias = append(l.Noticias, n)
		}
	}
	sort.SliceStable(l.Noticias, func(i, j int) bool { return depois(l.Noticias[i].PublicadaEm, l.Noticias[j].PublicadaEm) })
	return l, linhas.Err()
}

// depois: a mais nova primeiro; a sem data conta como a mais antiga de todas (fica no fim).
func depois(a, b *tipos.DataHora) bool { return tempo(a).After(tempo(b)) }

func tempo(d *tipos.DataHora) time.Time {
	if d == nil {
		return time.Time{}
	}
	return d.Tempo()
}

func (m *Modulo) listarHTTP(w http.ResponseWriter, r *http.Request) {
	l, err := m.Listar(r.Context())
	if err != nil {
		m.log.ErrorContext(r.Context(), "notícias: listar", "erro", err)
		respostas.SemCorpo(w, http.StatusInternalServerError)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, l)
}

// atualizarHTTP é o "Atualizar agora": no máximo uma vez a cada 5 minutos (429 antes disso). A falha de uma fonte não
// é erro do pedido: a lista volta com o erro na fonte.
func (m *Modulo) atualizarHTTP(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	agora := m.relogio.Agora()
	if !m.ultimaAMao.IsZero() && agora.Sub(m.ultimaAMao) < IntervaloAMao {
		m.mu.Unlock()
		respostas.Mensagem(w, http.StatusTooManyRequests, MensagemEspera)
		return
	}
	m.ultimaAMao = agora
	m.mu.Unlock()
	// O erro de cada fonte volta na lista (NoticiaFonte.Erro); o da tarefa de 45 minutos vai para Configurações › Erros.
	_ = m.Atualizar(r.Context())
	m.listarHTTP(w, r)
}
