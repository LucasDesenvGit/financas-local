package banco

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/banco/esquema"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/banco/esquema/embutido"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/relogio"
)

const (
	// BackupsGuardados é quantas cópias "antes da atualização" ficam; as mais antigas são apagadas.
	BackupsGuardados = 5
	// PrefixoBackup é o começo do nome das cópias, o mesmo do .NET.
	PrefixoBackup = "FinancasLocal-antes-atualizacao-"
)

// AplicarEsquema cria e atualiza o banco (ADR 0003): os scripts de banco/, embutidos no executável, e as colunas
// novas de esquema.ColunasNovas. Banco novo recebe também os dados iniciais. Num banco que já existe, se a assinatura
// dos scripts (PRAGMA user_version) mudou desde a última vez, faz antes uma cópia em dados/backups; se não mudou, não
// mexe em nada.
//
// Quem chama: o site, com -aplicar-esquema (a linha do serviço), e o subcomando aplicar-esquema, que os scripts de
// instalação rodam antes de publicar (cmd/financas/esquema.go, #723). A assinatura é a mesma, então um não refaz o
// trabalho do outro.
func AplicarEsquema(ctx context.Context, caminho string, r relogio.Relogio, log *slog.Logger) error {
	scripts := esquema.Scripts{
		Estrutura:     esquema.Texto(embutido.Estrutura),
		DadosIniciais: esquema.Texto(embutido.DadosIniciais),
		Colunas:       esquema.ColunasNovas,
	}
	return aplicarEsquema(ctx, caminho, scripts, r, log)
}

// AplicarEsquemaEDados é o que os scripts de instalação rodam (#723): o AplicarEsquema e, em seguida, o
// 02-dados-iniciais.sql sempre, também em banco que já existe, como o sqlite3 do 2-criar-banco.sh fazia. É assim que
// a vigência nova do INSS e do IRRF de cada ano chega a quem já tem banco (ADR 0065). O script é idempotente
// (INSERT OR IGNORE, WHERE NOT EXISTS). O site, ao iniciar, usa só o AplicarEsquema.
func AplicarEsquemaEDados(ctx context.Context, caminho string, r relogio.Relogio, log *slog.Logger) error {
	info, err := os.Stat(caminho)
	existia := err == nil && !info.IsDir() && info.Size() > 0
	if err := AplicarEsquema(ctx, caminho, r, log); err != nil {
		return err
	}
	if !existia { // o AplicarEsquema já pôs os dados iniciais no banco novo
		return nil
	}
	db, err := Abrir(ctx, caminho)
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, esquema.Texto(embutido.DadosIniciais)); err != nil {
		return fmt.Errorf("aplicar o 02-dados-iniciais.sql: %w", err)
	}
	return nil
}

func aplicarEsquema(ctx context.Context, caminhoBanco string, scripts esquema.Scripts, r relogio.Relogio, log *slog.Logger) error {
	colunas := scripts.Colunas
	assinatura := esquema.Assinatura(scripts.Estrutura, colunas)

	caminho, err := filepath.Abs(caminhoBanco)
	if err != nil {
		return fmt.Errorf("caminho do banco %s: %w", caminhoBanco, err)
	}
	// 0o777 menos o umask, como o Directory.CreateDirectory do .NET.
	if err := os.MkdirAll(filepath.Dir(caminho), 0o777); err != nil {
		return fmt.Errorf("criar a pasta do banco: %w", err)
	}
	// Como o File.Exists do .NET: qualquer erro ao olhar o arquivo, ou uma pasta no lugar dele, conta como "não existe".
	info, err := os.Stat(caminho)
	novo := err != nil || info.IsDir() || info.Size() == 0

	db, err := Abrir(ctx, caminho)
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(1) // uma conexão só, como o EsquemaBanco (Pooling = false)
	passarParaWal(ctx, db, log)

	if !novo {
		var versao int64
		if err := db.QueryRowContext(ctx, "PRAGMA user_version;").Scan(&versao); err != nil {
			return fmt.Errorf("ler o PRAGMA user_version: %w", err)
		}
		// Mesmos scripts da última vez: nada a fazer (o comum, a cada reinício do serviço).
		if versao == int64(assinatura) {
			return nil
		}
		if err := copiarAntesDaAtualizacao(ctx, db, caminho, r, log); err != nil {
			return err
		}
	}

	err = EmTransacao(ctx, db, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, scripts.Estrutura); err != nil {
			return fmt.Errorf("aplicar o 01-estrutura.sql: %w", err)
		}
		for _, c := range colunas {
			var existe int
			err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?;", c.Tabela, c.Coluna).Scan(&existe)
			if err != nil {
				return fmt.Errorf("conferir a coluna %s.%s: %w", c.Tabela, c.Coluna, err)
			}
			if existe > 0 {
				continue
			}
			// DDL não aceita parâmetro. O texto vem de esquema.ColunasNovas, nunca de quem usa o
			// site, e os nomes são só os da lista.
			alterar := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s;", c.Tabela, c.Coluna, c.Definicao)
			if _, err := tx.ExecContext(ctx, alterar); err != nil {
				return fmt.Errorf("acrescentar a coluna %s.%s: %w", c.Tabela, c.Coluna, err)
			}
			log.Info("Banco: coluna adicionada", "coluna", c.Coluna, "tabela", c.Tabela)
		}
		// Dados iniciais só no banco novo: rodar de novo recriaria categorias que a pessoa apagou.
		if novo {
			if _, err := tx.ExecContext(ctx, scripts.DadosIniciais); err != nil {
				return fmt.Errorf("aplicar o 02-dados-iniciais.sql: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d;", assinatura)); err != nil {
		return fmt.Errorf("gravar o PRAGMA user_version: %w", err)
	}
	if novo {
		log.Info("Banco criado", "caminho", caminho)
	} else {
		log.Info("Estrutura do banco conferida", "caminho", caminho)
	}
	return nil
}

// passarParaWal faz o mesmo que o EsquemaBanco.PassarParaWal do .NET (#231, ADR 0049): o .NET e o Go usam o mesmo
// arquivo, e em WAL quem lê não espera quem grava. O modo fica gravado no arquivo; rodar de novo não muda nada. Só
// dá para trocar sem outra conexão aberta: se não deu (ex.: o DB Browser aberto), avisa e segue no modo de antes.
func passarParaWal(ctx context.Context, db *sql.DB, log *slog.Logger) {
	var modo string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode = WAL;").Scan(&modo); err != nil {
		modo = err.Error()
	}
	if !strings.EqualFold(modo, "wal") {
		log.Warn("Banco: não deu para passar para o modo WAL; feche outros programas com o banco aberto e reinicie o serviço", "modo", modo)
	}
}

// pastaDasCopias é dados/backups quando o banco está em dados/sql (sem diferença de maiúsculas no "sql"), e senão a
// pasta backups ao lado do banco.
func pastaDasCopias(caminho string) string {
	pastaSql := filepath.Dir(caminho)
	pastaDados := pastaSql
	if strings.EqualFold(filepath.Base(pastaSql), "sql") {
		pastaDados = filepath.Dir(pastaSql)
	}
	return filepath.Join(pastaDados, "backups")
}

// copiarAntesDaAtualizacao faz uma cópia consistente (VACUUM INTO) em dados/backups e guarda as 5 mais recentes.
//
// O nome leva a data e a hora de Brasília (relogio). O .NET usa o DateTime.Now, o fuso do computador: num
// computador fora de Brasília os nomes dos dois lados diferem em algumas horas.
func copiarAntesDaAtualizacao(ctx context.Context, db *sql.DB, caminho string, r relogio.Relogio, log *slog.Logger) error {
	pasta := pastaDasCopias(caminho)
	if err := os.MkdirAll(pasta, 0o777); err != nil {
		return fmt.Errorf("criar a pasta das cópias: %w", err)
	}
	destino := filepath.Join(pasta, nomeDaCopia(relogio.AgoraLocal(r)))
	if _, err := db.ExecContext(ctx, "VACUUM INTO ?;", destino); err != nil {
		return fmt.Errorf("copiar o banco antes da atualização para %s: %w", destino, err)
	}
	log.Info("Banco: cópia antes da atualização", "destino", destino)
	return apagarCopiasAntigas(pasta, log)
}

// nomeDaCopia é FinancasLocal-antes-atualizacao-AAAAMMDD_HHMMSS_mmm.db, o mesmo yyyyMMdd_HHmmss_fff do .NET.
//
// #366: com o nome só até o segundo, duas atualizações no mesmo segundo (a API reiniciando logo depois de uma
// atualização que falhou) davam o mesmo nome, e o VACUUM INTO recusa um destino que já existe. Os milissegundos são
// cortados, não arredondados, como o fff do .NET. O "_" vem depois do "." na tabela ASCII: o nome antigo de um
// segundo (…_HHMMSS.db) fica antes dos novos do mesmo segundo, e a ordem pelo nome continua a ordem do tempo.
func nomeDaCopia(instante time.Time) string {
	return fmt.Sprintf("%s%s_%03d.db", PrefixoBackup, instante.Format("20060102_150405"), instante.Nanosecond()/int(time.Millisecond))
}

// apagarCopiasAntigas deixa só as BackupsGuardados cópias mais recentes pelo nome (a data e a hora estão nele),
// comparado byte a byte, como o StringComparer.Ordinal do .NET.
//
// #365: uma cópia que não dá para apagar (outro dono, só leitura, aberta noutro programa) fica para a próxima, com
// um aviso, e não impede a API de subir.
func apagarCopiasAntigas(pasta string, log *slog.Logger) error {
	entradas, err := os.ReadDir(pasta)
	if err != nil {
		return fmt.Errorf("listar as cópias em %s: %w", pasta, err)
	}
	var copias []string
	for _, e := range entradas {
		nome := e.Name()
		// O padrão "FinancasLocal-antes-atualizacao-*.db" do GetFiles: só arquivos, e o * pode ser vazio.
		if !e.IsDir() && strings.HasPrefix(nome, PrefixoBackup) && strings.HasSuffix(nome, ".db") {
			copias = append(copias, nome)
		}
	}
	slices.Sort(copias)
	slices.Reverse(copias) // da mais recente para a mais antiga
	for _, antigo := range copias[min(BackupsGuardados, len(copias)):] {
		if err := apagarCopia(filepath.Join(pasta, antigo)); err != nil {
			log.Warn("Banco: não deu para apagar uma cópia antiga; fica para a próxima", "arquivo", antigo, "erro", err)
		}
	}
	return nil
}

// apagarCopia apaga o arquivo. Um arquivo que já não existe não é erro, como no FileInfo.Delete do .NET.
func apagarCopia(caminho string) error {
	if err := os.Remove(caminho); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
