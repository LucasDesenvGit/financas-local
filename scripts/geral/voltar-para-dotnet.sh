#!/usr/bin/env bash
# =====================================================================
# FinancasLocal - voltar o servico para o .NET (#85, ADR 0049)
#
# Depois da troca (3-publicar-atualizar.sh --backend go), o Go fica na frente. Este script poe o .NET de volta, sem
# restaurar o banco: os dois gravam o mesmo formato (ADR 0049). O .NET continua publicado ao lado do Go ate a #86.
#
# O que o .NET nao le: os segredos que o Go gravou ("v2:", internal/segredos). Com eles, a sincronizacao falharia a
# cada ciclo com a tela dizendo "configurado". Por isso o script os apaga (o Client Secret da Pluggy e o token da
# brapi), como o MigrarSqlite faz, e avisa que e preciso informa-los de novo. Os outros dados ficam como estao.
#
# Uso (no Windows, num Git Bash aberto como Administrador, como o instalar.sh):
#   scripts/geral/voltar-para-dotnet.sh
# =====================================================================
source "$(dirname "${BASH_SOURCE[0]}")/comum.sh"

banco="$PastaSql/FinancasLocal.db"
[[ -f "$banco" ]] || erro_sai "Banco nao encontrado em $banco."
command -v sqlite3 >/dev/null 2>&1 || erro_sai 'sqlite3 nao encontrado. Rode antes o 1-preparar-ambiente.sh.'

etapa 'Segredos que so o Go le'
chaves="'Pluggy.ClientSecret', 'Cotacao.BrapiToken'"
apagados="$(sqlite3 "${SQLITE_ESPERA[@]}" "$banco" \
    "UPDATE Configuracao SET Valor = NULL, Protegido = 0 WHERE Chave IN ($chaves) AND Valor LIKE 'v2:%'; SELECT changes();")"
if [[ "${apagados:-0}" -gt 0 ]]; then
    aviso "$apagados segredo(s) gravado(s) pelo Go foram apagados: o .NET nao os le."
    aviso 'Informe de novo em Configuracoes: o Client Secret da Pluggy (Open Finance) e o token da brapi (Investimentos).'
else
    ok 'nenhum segredo do Go a apagar'
fi

etapa 'Voltando o servico para o .NET'
bash "$(dirname "${BASH_SOURCE[0]}")/3-publicar-atualizar.sh" --pular-build --backend dotnet
