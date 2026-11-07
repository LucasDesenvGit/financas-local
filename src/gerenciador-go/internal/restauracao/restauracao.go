// Package restauracao é o "Restaurar uma cópia" do gerenciador (#533): troca o banco do site por uma cópia, com os
// passos do docs/OPERACAO.md ("Restaurar uma cópia"), mais o que o guia deixa por conta de quem digita os comandos:
// conferir que o arquivo é mesmo um banco do Finanças Local e voltar o banco de antes se algo der errado no meio.
package restauracao

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

var (
	// ErrSemBanco: o banco do site não está onde a instalação diz.
	ErrSemBanco = errors.New("o banco do site não foi encontrado")
	// ErrNaoEhCopia: o arquivo escolhido não é um banco do Finanças Local (ou está estragado).
	ErrNaoEhCopia = errors.New("o arquivo não é uma cópia do banco do Finanças Local")
	// ErrMesmoArquivo: o arquivo escolhido é o próprio banco do site.
	ErrMesmoArquivo = errors.New("o arquivo escolhido é o próprio banco do site")
)

// PastaAntes é onde o banco de antes fica guardado, dentro da pasta dados (a mesma do guia de operação).
const PastaAntes = "antes-de-restaurar"

// Servico é o que a restauração usa do serviço do site: desligar antes de trocar o banco e ligar depois.
type Servico interface {
	Desligar() error
	Ligar() error
}

// Conferir abre a cópia só para leitura e confere que ela é um banco SQLite inteiro (quick_check) com as tabelas do
// Finanças Local. Roda antes de desligar o site: um arquivo errado não derruba nada.
func Conferir(copia string) error {
	if _, err := os.Stat(copia); err != nil {
		return fmt.Errorf("%w: %v", ErrNaoEhCopia, err)
	}
	parametros := url.Values{}
	parametros.Add("mode", "ro")
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(copia)+"?"+parametros.Encode())
	if err != nil {
		return fmt.Errorf("%w: %v", ErrNaoEhCopia, err)
	}
	defer db.Close()
	var resultado string
	if err := db.QueryRow("PRAGMA quick_check").Scan(&resultado); err != nil {
		return fmt.Errorf("%w: %v", ErrNaoEhCopia, err)
	}
	if resultado != "ok" {
		return fmt.Errorf("%w: quick_check %q", ErrNaoEhCopia, resultado)
	}
	var tabelas int
	err = db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name IN ('Transacao', 'Conta', 'Usuario')`).
		Scan(&tabelas)
	if err != nil || tabelas != 3 {
		return fmt.Errorf("%w: sem as tabelas do site", ErrNaoEhCopia)
	}
	return nil
}

// Restaurar troca o banco pela cópia, nesta ordem:
//  1. confere a cópia (Conferir) e que ela não é o próprio banco;
//  2. desliga o site;
//  3. guarda o banco de agora, com o -wal e o -shm, em dados/antes-de-restaurar (trocando o que já estiver lá);
//  4. põe a cópia no lugar do banco e apaga o -wal e o -shm, que são do banco antigo: ao lado da cópia, eles a
//     corromperiam (https://www.sqlite.org/howtocorrupt.html);
//  5. liga o site.
//
// Se o passo 4 falhar no meio, devolve o banco de antes (guardado no passo 3); se o 3 falhar, o banco ainda não foi
// mexido. Nos dois casos, liga o site de novo. Devolve o caminho de antes-de-restaurar.
func Restaurar(banco, copia string, s Servico) (string, error) {
	if _, err := os.Stat(banco); err != nil {
		return "", fmt.Errorf("%w: %s", ErrSemBanco, banco)
	}
	if err := Conferir(copia); err != nil {
		return "", err
	}
	if mesmo, err := mesmoArquivo(banco, copia); err != nil || mesmo {
		return "", ErrMesmoArquivo
	}
	antes := filepath.Join(filepath.Dir(filepath.Dir(banco)), PastaAntes)
	if err := s.Desligar(); err != nil {
		return "", fmt.Errorf("desligar o site: %w", err)
	}
	if mexeu, err := trocar(banco, copia, antes); err != nil {
		if mexeu {
			if errVolta := voltar(banco, antes); errVolta != nil {
				err = errors.Join(err, fmt.Errorf("voltar o banco de antes: %w", errVolta))
			}
		}
		return antes, errors.Join(err, ligar(s))
	}
	return antes, ligar(s)
}

func ligar(s Servico) error {
	if err := s.Ligar(); err != nil {
		return fmt.Errorf("ligar o site: %w", err)
	}
	return nil
}

func mesmoArquivo(a, b string) (bool, error) {
	ia, err := os.Stat(a)
	if err != nil {
		return false, err
	}
	ib, err := os.Stat(b)
	if err != nil {
		return false, err
	}
	return os.SameFile(ia, ib), nil
}

// renomear é o os.Rename; os testes o trocam para falhar depois de o banco já ter sido mexido.
var renomear = os.Rename

// trocar guarda o banco de agora em antes e põe a cópia no lugar. A cópia vai primeiro para um arquivo ao lado do
// banco e só então troca de nome: o banco nunca fica pela metade. mexeu diz se o banco já tinha sido mexido quando deu
// erro (só depois de guardado o de antes e copiada a cópia).
func trocar(banco, copia, antes string) (mexeu bool, err error) {
	if err := os.MkdirAll(antes, 0o700); err != nil {
		return false, fmt.Errorf("criar %s: %w", antes, err)
	}
	for _, sufixo := range []string{"", "-wal", "-shm"} {
		destino := filepath.Join(antes, filepath.Base(banco)+sufixo)
		if err := os.Remove(destino); err != nil && !errors.Is(err, os.ErrNotExist) {
			return false, err
		}
		if err := copiarArquivo(banco+sufixo, destino); err != nil && !(sufixo != "" && errors.Is(err, os.ErrNotExist)) {
			return false, fmt.Errorf("guardar o banco de antes: %w", err)
		}
	}
	temporario := banco + ".restaurando"
	if err := copiarArquivo(copia, temporario); err != nil {
		os.Remove(temporario)
		return false, fmt.Errorf("copiar a cópia: %w", err)
	}
	// Daqui em diante, o banco muda.
	for _, sufixo := range []string{"-wal", "-shm"} {
		if err := os.Remove(banco + sufixo); err != nil && !errors.Is(err, os.ErrNotExist) {
			os.Remove(temporario)
			return true, fmt.Errorf("apagar o %s do banco antigo: %w", sufixo, err)
		}
	}
	if err := renomear(temporario, banco); err != nil {
		os.Remove(temporario)
		return true, fmt.Errorf("pôr a cópia no lugar: %w", err)
	}
	return false, nil
}

// voltar devolve o banco de antes (e o -wal e o -shm dele, que fazem parte do banco). Sem o banco guardado, não mexe
// em nada.
func voltar(banco, antes string) error {
	if _, err := os.Stat(filepath.Join(antes, filepath.Base(banco))); err != nil {
		return fmt.Errorf("o banco de antes não está em %s: %w", antes, err)
	}
	for _, sufixo := range []string{"", "-wal", "-shm"} {
		origem := filepath.Join(antes, filepath.Base(banco)+sufixo)
		if _, err := os.Stat(origem); errors.Is(err, os.ErrNotExist) {
			if err := os.Remove(banco + sufixo); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			continue
		}
		if err := copiarArquivo(origem, banco+sufixo); err != nil {
			return err
		}
	}
	return nil
}

// copiarArquivo copia o conteúdo, criando o destino só para o dono (o banco tem todos os dados financeiros; no
// Windows, o modo não muda nada, e a pasta dados já é só do serviço e dos administradores).
func copiarArquivo(origem, destino string) error {
	de, err := os.Open(origem)
	if err != nil {
		return err
	}
	defer de.Close()
	para, err := os.OpenFile(destino, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(para, de); err != nil {
		para.Close()
		return err
	}
	if err := para.Sync(); err != nil {
		para.Close()
		return err
	}
	return para.Close()
}
