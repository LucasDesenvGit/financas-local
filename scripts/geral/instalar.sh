#!/usr/bin/env bash
# =====================================================================
# FinancasLocal - instalacao/atualizacao completa em um comando
#
# Roda em sequencia, parando no primeiro erro:
#   1-preparar-ambiente.sh   (.NET, Node.js e sqlite3)
#   2-criar-banco.sh         (cria ou atualiza o banco; nao apaga dados)
#   3-publicar-atualizar.sh  (compila, publica e registra o servico)
#   No Linux, no fim, o gerenciador (scripts/linux/instalar-gerenciador.sh),
#   a janela que mostra se o site esta no ar e liga e desliga (#300).
#
# Pode ser executado de novo para atualizar: as tres etapas sao idempotentes.
# No Windows, o instalar-windows.cmd (na raiz) abre este script no Git Bash
# como Administrador.
#
# Ao migrar de uma instalacao com SQL Server, NAO use este script: siga a
# secao "Migrar de uma instalacao com SQL Server" de docs/INSTALACAO-PELO-CODIGO.md.
#
# Uso:
#   scripts/geral/instalar.sh
#   scripts/geral/instalar.sh --pular-build   # repassado ao 3-publicar-atualizar.sh
#   scripts/geral/instalar.sh --sem-gerenciador   # Linux: nao instala o gerenciador
# =====================================================================
source "$(dirname "${BASH_SOURCE[0]}")/comum.sh"
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# --sem-gerenciador e daqui; o resto vai para o 3-publicar-atualizar.sh.
com_gerenciador=1
argumentos_publicar=()
for argumento in "$@"; do
    if [[ "$argumento" == --sem-gerenciador ]]; then com_gerenciador=0; else argumentos_publicar+=("$argumento"); fi
done

if [[ "$SO" == windows ]] && ! net session >/dev/null 2>&1; then
    aviso 'Este Git Bash nao esta como Administrador: o servico do Windows provavelmente nao sera registrado.'
    aviso 'Para evitar, use o instalar-windows.cmd ou abra o Git Bash como Administrador.'
fi

etapa '[1/3] Preparando o ambiente'
bash "$script_dir/1-preparar-ambiente.sh"

etapa '[2/3] Criando/atualizando o banco'
bash "$script_dir/2-criar-banco.sh"

etapa '[3/3] Publicando'
bash "$script_dir/3-publicar-atualizar.sh" "${argumentos_publicar[@]+"${argumentos_publicar[@]}"}"

# #300: o gerenciador vinha num passo a parte, e muita gente nem sabia dele.
if [[ "$SO" == linux && "$com_gerenciador" -eq 1 ]]; then
    etapa 'Gerenciador do Financas Local'
    if bash "$script_dir/../linux/instalar-gerenciador.sh"; then
        ok 'gerenciador instalado (menu de aplicativos: Gerenciador do Financas Local)'
    else
        aviso 'Nao foi possivel instalar o gerenciador; o site esta instalado. Para tentar de novo: scripts/linux/instalar-gerenciador.sh'
    fi
elif [[ "$SO" == windows ]]; then
    echo ''
    echo 'Gerenciador (ver se o site esta no ar, ligar, desligar, copiar o banco):'
    echo '   powershell -ExecutionPolicy Bypass -File scripts\windows\instalar-gerenciador.ps1'
fi

echo ''
echo -e "\033[32mInstalacao concluida. Abra https://localhost:5443 no navegador.\033[0m"
