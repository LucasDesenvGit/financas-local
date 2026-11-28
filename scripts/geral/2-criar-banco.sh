#!/usr/bin/env bash
# =====================================================================
# FinancasLocal - 2) Criacao do banco de dados
#
#  - Cria as pastas de dados com permissoes restritas (melhor esforco)
#  - Cria a estrutura, as colunas novas e os dados iniciais pelo executavel do
#    Go (subcomando aplicar-esquema, #723); no banco existente, so o que falta
#  - Pode ser executado novamente: o esquema e idempotente
#
# Uso:
#   scripts/geral/2-criar-banco.sh                           # usa dados/sql/FinancasLocal.db
#   scripts/geral/2-criar-banco.sh /caminho/outro/banco.db    # caminho customizado
# =====================================================================
source "$(dirname "${BASH_SOURCE[0]}")/comum.sh"
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

requer_comando sqlite3 'sqlite3 nao encontrado no PATH. Rode antes o 1-preparar-ambiente.sh.'

banco="${1:-$PastaSql/FinancasLocal.db}"
# O go run roda dentro de src/backend: um caminho relativo valeria a partir de la.
[[ "$banco" == /* || "$banco" =~ ^[A-Za-z]:[/\\] ]] || banco="$PWD/$banco"

etapa 'Criando pastas de dados'
garantir_pasta "$Dados"
garantir_pasta "$PastaSql"
garantir_pasta "$PastaChaves"
garantir_pasta "$PastaBackups"
# chmod 700 vale no Linux; no Git Bash/Windows nao faz nada: la quem protege as
# pastas (ACL do NTFS) e o instalar-windows.ps1 (Proteger-Pasta, #68).
chmod 700 "$Dados" 2>/dev/null || true
chmod 700 "$PastaBackups" 2>/dev/null || true
ok 'pastas criadas e protegidas (melhor esforco)'

# Atualizacao: copia o banco antes de mexer na estrutura. .backup e seguro com a API no ar.
# Ficam as 5 copias mais recentes (fora da rotacao do 4-backup.sh, que usa outro nome).
if [[ -f "$banco" ]]; then
    etapa 'Backup do banco antes da atualizacao'
    copia="$PastaBackups/FinancasLocal-antes-instalacao-$(date +%Y%m%d_%H%M%S).db"
    sqlite3 "${SQLITE_ESPERA[@]}" "$banco" ".backup \"$(caminho_sqlite "$copia")\""
    # shellcheck disable=SC2012
    ls -1 "$PastaBackups"/FinancasLocal-antes-instalacao-*.db 2>/dev/null | sort -r | tail -n +6 | while IFS= read -r velho; do rm -f "$velho"; done
    ok "copia em $copia"
fi

etapa "Criando/atualizando a estrutura em $banco"
# #723: quem aplica o esquema e o executavel do Go (banco/01-estrutura.sql e 02-dados-iniciais.sql embutidos, as
# colunas novas em internal/plataforma/banco/esquema/colunas_novas.go): banco novo nasce completo e banco antigo ganha
# o que falta, sem repetir o que ja existe. O executavel ainda nao foi publicado nesta etapa (a 3 faz isso), entao
# roda do codigo com o Go da etapa 1: o cache de compilacao evita compilar tudo de novo na etapa 3.
configurar_ambiente_build
command -v go >/dev/null 2>&1 || erro_sai 'Go nao encontrado. Rode antes o 1-preparar-ambiente.sh.'
(cd "$Raiz/src/backend" && go run ./cmd/financas aplicar-esquema -banco "$(caminho_nativo "$banco")") \
    || erro_sai 'Nao foi possivel criar/atualizar o banco.'
ok 'estrutura, colunas novas e dados iniciais conferidos'

total_tabelas="$(sqlite3 "${SQLITE_ESPERA[@]}" "$banco" "SELECT COUNT(*) FROM sqlite_master WHERE type='table';")"

echo ''
echo -e "\033[32mBanco pronto ($total_tabelas tabela(s)). Proximo passo:\033[0m"
echo "   $script_dir/3-publicar-atualizar.sh"
