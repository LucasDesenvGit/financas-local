package openfinance

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/auditoria"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco/consultas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/transferencias"
)

// configuracaoDto é o ConfiguracaoPluggyDto. O Client Secret nunca sai: só se ele existe.
type configuracaoDto struct {
	ClientId          *string `json:"clientId"`
	SecretConfigurado bool    `json:"secretConfigurado"`
	IntervaloHoras    int     `json:"intervaloHoras"`
	DiasRetroativos   int     `json:"diasRetroativos"`
	Habilitada        bool    `json:"habilitada"`
	MeuNome           *string `json:"meuNome"`
}

func (s *Servico) lerConfiguracao(ctx context.Context) (d configuracaoDto, err error) {
	if d.ClientId, err = s.config.Obter(ctx, chaveClientId); err != nil {
		return d, err
	}
	if d.SecretConfigurado, err = s.config.Existe(ctx, chaveClientSecret); err != nil {
		return d, err
	}
	if d.IntervaloHoras, err = s.config.ObterInteiro(ctx, chaveIntervaloHoras, 6); err != nil {
		return d, err
	}
	if d.DiasRetroativos, err = s.config.ObterInteiro(ctx, chaveDiasRetroativos, 30); err != nil {
		return d, err
	}
	if d.Habilitada, err = s.config.ObterBool(ctx, chaveHabilitada, true); err != nil {
		return d, err
	}
	d.MeuNome, err = s.config.Obter(ctx, transferencias.ChaveMeuNome)
	return d, err
}

func (s *Servico) obterConfiguracao(w http.ResponseWriter, r *http.Request) {
	d, err := s.lerConfiguracao(r.Context())
	if err != nil {
		s.erroInterno(w, r, "ler a configuração", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, d)
}

// salvarConfiguracao grava as credenciais. O Client Secret vai cifrado; os campos opcionais, omitidos, ficam como estão.
func (s *Servico) salvarConfiguracao(w http.ResponseWriter, r *http.Request) {
	clientId := respostas.CampoTexto{Propriedade: "ClientId", Maximo: 100}
	clientSecret := respostas.CampoTexto{Propriedade: "ClientSecret", Maximo: 200, Opcional: true}
	intervalo := respostas.CampoInteiroOpcional{Propriedade: "IntervaloHoras", Faixa: &respostas.Faixa{Minimo: 1, Maximo: 48}}
	dias := respostas.CampoInteiroOpcional{Propriedade: "DiasRetroativos", Faixa: &respostas.Faixa{Minimo: 1, Maximo: 365}}
	habilitada := respostas.CampoBoolOpcional{Propriedade: "Habilitada"}
	if !respostas.LerObjeto(w, r, "dto", "Financas.Api.Dtos.SalvarConfiguracaoPluggyDto",
		&clientId, &clientSecret, &intervalo, &dias, &habilitada) {
		return
	}
	ctx := r.Context()
	login, err := auditoria.Login(ctx, consultas.New(s.db))
	if err != nil {
		s.erroInterno(w, r, "ler o login", err)
		return
	}
	err = banco.EmTransacao(ctx, s.db, func(tx *sql.Tx) error {
		config := s.config.ComTx(tx)
		id := strings.TrimSpace(*clientId.Valor)
		if err := config.Definir(ctx, chaveClientId, &id); err != nil {
			return err
		}
		if clientSecret.Valor != nil && strings.TrimSpace(*clientSecret.Valor) != "" {
			segredo := strings.TrimSpace(*clientSecret.Valor)
			if err := config.DefinirSegredo(ctx, s.cofre, chaveClientSecret, &segredo); err != nil {
				return err
			}
			if err := auditoria.Registrar(ctx, consultas.New(tx), r, s.agora(), "CredencialPluggyAlterada", login); err != nil {
				return err
			}
		}
		if intervalo.Valor != nil {
			texto := strconv.FormatInt(*intervalo.Valor, 10)
			if err := config.Definir(ctx, chaveIntervaloHoras, &texto); err != nil {
				return err
			}
		}
		if dias.Valor != nil {
			texto := strconv.FormatInt(*dias.Valor, 10)
			if err := config.Definir(ctx, chaveDiasRetroativos, &texto); err != nil {
				return err
			}
		}
		if habilitada.Valor != nil {
			texto := strconv.FormatBool(*habilitada.Valor)
			return config.Definir(ctx, chaveHabilitada, &texto)
		}
		return nil
	})
	if err != nil {
		s.erroInterno(w, r, "gravar a configuração", err)
		return
	}
	respostas.SemCorpo(w, http.StatusNoContent)
}

// removerSecret apaga a linha do Client Secret, e não só o valor: é o ExecuteDelete do .NET.
func (s *Servico) removerSecret(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	login, err := auditoria.Login(ctx, consultas.New(s.db))
	if err != nil {
		s.erroInterno(w, r, "ler o login", err)
		return
	}
	err = banco.EmTransacao(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM "Configuracao" WHERE "Chave" = ?`, chaveClientSecret); err != nil {
			return err
		}
		return auditoria.Registrar(ctx, consultas.New(tx), r, s.agora(), "CredencialPluggyRemovida", login)
	})
	if err != nil {
		s.erroInterno(w, r, "remover o Client Secret", err)
		return
	}
	respostas.SemCorpo(w, http.StatusNoContent)
}

// salvarMeuNome grava o seu nome como aparece nos Pix (#172, ADR 0031) e reclassifica as transferências na hora. Vazio apaga.
func (s *Servico) salvarMeuNome(w http.ResponseWriter, r *http.Request) {
	nome := respostas.CampoTexto{Propriedade: "Nome", Maximo: 120, Opcional: true, MensagemDoTamanho: "O nome pode ter até 120 caracteres."}
	if !respostas.LerObjeto(w, r, "dto", "Financas.Api.Dtos.MeuNomeDto", &nome) {
		return
	}
	var valor *string
	if nome.Valor != nil && strings.TrimSpace(*nome.Valor) != "" {
		aparado := strings.TrimSpace(*nome.Valor)
		valor = &aparado
	}
	if valor != nil && transferencias.PalavrasDoNome(valor) == nil {
		respostas.Mensagem(w, http.StatusBadRequest, "Informe pelo menos o primeiro e o último nome, como aparecem nos Pix.")
		return
	}
	ctx := r.Context()
	if err := s.config.Definir(ctx, transferencias.ChaveMeuNome, valor); err != nil {
		s.erroInterno(w, r, "gravar o nome", err)
		return
	}
	err := banco.EmTransacao(ctx, s.db, func(tx *sql.Tx) error {
		_, err := transferencias.Reclassificar(ctx, tx, s.agora())
		return err
	})
	if err != nil {
		s.erroInterno(w, r, "reclassificar as transferências", err)
		return
	}
	respostas.SemCorpo(w, http.StatusNoContent)
}
