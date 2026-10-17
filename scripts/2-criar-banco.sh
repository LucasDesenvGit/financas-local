#!/usr/bin/env bash
# =====================================================================
# FinancasLocal - 2) Criacao do banco de dados
#
#  - Cria as pastas de dados com permissoes restritas (melhor esforco)
#  - Cria a estrutura (banco/01-estrutura.sql) e insere os dados iniciais
#    (banco/02-dados-iniciais.sql) via sqlite3
#  - Pode ser executado novamente: os scripts SQL sao idempotentes
#
# Uso:
#   scripts/2-criar-banco.sh                           # usa dados/sql/FinancasLocal.db
#   scripts/2-criar-banco.sh /caminho/outro/banco.db    # caminho customizado
# =====================================================================
source "$(dirname "${BASH_SOURCE[0]}")/comum.sh"
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

requer_comando sqlite3 'sqlite3 nao encontrado no PATH. Rode antes o 1-preparar-ambiente.sh.'

banco="${1:-$PastaSql/FinancasLocal.db}"
pasta_banco_sql="$Raiz/banco"

# Convencao para FUTURAS alteracoes de schema: veja os helpers coluna_existe e
# adicionar_coluna_se_ausente em comum.sh (dependem da variavel $banco acima).

etapa 'Criando pastas de dados'
garantir_pasta "$Dados"
garantir_pasta "$PastaSql"
garantir_pasta "$PastaChaves"
garantir_pasta "$PastaBackups"
# chmod 700 e melhor esforco: no Git Bash/Windows isso tem efeito limitado (nao
# ha conceito de dono/modo como no Linux); a protecao real por la seria via
# ACLs do NTFS, o que ficou fora de escopo agora que nao existe mais uma
# identidade de pool de aplicativo do IIS para quem conceder acesso.
chmod 700 "$Dados" 2>/dev/null || true
chmod 700 "$PastaBackups" 2>/dev/null || true
ok 'pastas criadas e protegidas (melhor esforco)'

# Atualizacao: copia o banco antes de mexer na estrutura. .backup e seguro com a API no ar.
# Ficam as 5 copias mais recentes (fora da rotacao do 4-backup.sh, que usa outro nome).
if [[ -f "$banco" ]]; then
    etapa 'Backup do banco antes da atualizacao'
    copia="$PastaBackups/FinancasLocal-antes-instalacao-$(date +%Y%m%d_%H%M%S).db"
    # O shell do sqlite3 so tem escape dentro de "...": \ e \" (apostrofo ou aspas no caminho).
    copia_sql=${copia//\\/\\\\}
    copia_sql=${copia_sql//\"/\\\"}
    sqlite3 "${SQLITE_ESPERA[@]}" "$banco" ".backup \"$copia_sql\""
    # shellcheck disable=SC2012
    ls -1 "$PastaBackups"/FinancasLocal-antes-instalacao-*.db 2>/dev/null | sort -r | tail -n +6 | while IFS= read -r velho; do rm -f "$velho"; done
    ok "copia em $copia"
fi

etapa "Criando a estrutura em $banco (01-estrutura.sql)"
[[ -f "$pasta_banco_sql/01-estrutura.sql" ]] || erro_sai "Arquivo nao encontrado: $pasta_banco_sql/01-estrutura.sql"
sqlite3 "${SQLITE_ESPERA[@]}" "$banco" < "$pasta_banco_sql/01-estrutura.sql"
ok 'estrutura criada/atualizada'

etapa 'Adicionando colunas novas em tabelas existentes'
# Bancos criados antes de cada coluna existir; em banco novo ela ja nasce no CREATE TABLE.
adicionar_coluna_se_ausente "Ativo" "Moeda" "TEXT NOT NULL DEFAULT 'BRL'"
adicionar_coluna_se_ausente "OperacaoAtivo" "Cambio" "TEXT NOT NULL DEFAULT '1'"
adicionar_coluna_se_ausente "Meta" "ReservaNome" "TEXT NULL"
adicionar_coluna_se_ausente "Meta" "Prioridade" "INTEGER NOT NULL DEFAULT 0"
adicionar_coluna_se_ausente "ContaAgendada" "DataLimite" "TEXT NULL"
ok 'colunas conferidas'

etapa 'Inserindo dados iniciais (02-dados-iniciais.sql)'
[[ -f "$pasta_banco_sql/02-dados-iniciais.sql" ]] || erro_sai "Arquivo nao encontrado: $pasta_banco_sql/02-dados-iniciais.sql"
sqlite3 "${SQLITE_ESPERA[@]}" "$banco" < "$pasta_banco_sql/02-dados-iniciais.sql"
ok 'categorias e configuracoes padrao'

total_tabelas="$(sqlite3 "${SQLITE_ESPERA[@]}" "$banco" "SELECT COUNT(*) FROM sqlite_master WHERE type='table';")"

echo ''
echo -e "\033[32mBanco pronto ($total_tabelas tabela(s)). Proximo passo:\033[0m"
echo "   $script_dir/3-publicar-atualizar.sh"
