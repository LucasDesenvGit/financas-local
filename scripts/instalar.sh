#!/usr/bin/env bash
# =====================================================================
# FinancasLocal - instalacao/atualizacao completa em um comando
#
# Roda em sequencia, parando no primeiro erro:
#   1-preparar-ambiente.sh   (.NET, Node.js e sqlite3)
#   2-criar-banco.sh         (cria ou atualiza o banco; nao apaga dados)
#   3-publicar-atualizar.sh  (compila, publica e registra o servico)
#
# Pode ser executado de novo para atualizar: as tres etapas sao idempotentes.
# No Windows, o instalar-windows.cmd (na raiz) abre este script no Git Bash
# como Administrador.
#
# Ao migrar de uma instalacao com SQL Server, NAO use este script: siga a
# secao "Migrar de uma instalacao com SQL Server" do README.
#
# Uso:
#   scripts/instalar.sh
#   scripts/instalar.sh --pular-build   # repassado ao 3-publicar-atualizar.sh
# =====================================================================
source "$(dirname "${BASH_SOURCE[0]}")/comum.sh"
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if [[ "$SO" == windows ]] && ! net session >/dev/null 2>&1; then
    aviso 'Este Git Bash nao esta como Administrador: o servico do Windows provavelmente nao sera registrado.'
    aviso 'Para evitar, use o instalar-windows.cmd ou abra o Git Bash como Administrador.'
fi

etapa '[1/3] Preparando o ambiente'
bash "$script_dir/1-preparar-ambiente.sh"

etapa '[2/3] Criando/atualizando o banco'
bash "$script_dir/2-criar-banco.sh"

etapa '[3/3] Publicando'
bash "$script_dir/3-publicar-atualizar.sh" "$@"

echo ''
echo -e "\033[32mInstalacao concluida. Abra https://localhost:5443 no navegador.\033[0m"
