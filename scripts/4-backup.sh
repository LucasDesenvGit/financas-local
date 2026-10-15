#!/usr/bin/env bash
# =====================================================================
# FinancasLocal - Backup do banco de dados
#
# Gera backups/FinancasLocal_AAAAMMDD_HHMMSS.db com "sqlite3 .backup" (backup
# online, seguro mesmo com o banco em uso) e mantem somente os N mais
# recentes.
#
# Uso:
#   scripts/4-backup.sh                     # backup agora
#   scripts/4-backup.sh --manter 30
#   scripts/4-backup.sh --agendar           # agenda backup diario as 12:30
#   scripts/4-backup.sh --agendar --horario 23:00
#
# Dica: copie a pasta backups para um disco externo ou nuvem de confianca de
# tempos em tempos. Para restaurar, com o servico parado, copie o .db do
# backup para dados/sql/FinancasLocal.db.
# =====================================================================
source "$(dirname "${BASH_SOURCE[0]}")/comum.sh"

requer_comando sqlite3 'sqlite3 nao encontrado no PATH. Rode antes o 1-preparar-ambiente.sh.'

banco="$PastaSql/FinancasLocal.db"
manter=14
agendar=0
horario='12:30'

while [[ $# -gt 0 ]]; do
    case "$1" in
        --manter)
            [[ $# -ge 2 ]] || erro_sai 'Faltou o valor de --manter.'
            manter="$2"
            shift 2
            ;;
        --agendar)
            agendar=1
            shift
            ;;
        --horario)
            [[ $# -ge 2 ]] || erro_sai 'Faltou o valor de --horario.'
            horario="$2"
            shift 2
            ;;
        *)
            erro_sai "Argumento desconhecido: $1"
            ;;
    esac
done

[[ "$manter" =~ ^[0-9]+$ ]] && [[ "$manter" -ge 1 ]] && [[ "$manter" -le 365 ]] || erro_sai '--manter precisa ser um numero entre 1 e 365.'

script_path="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/$(basename "${BASH_SOURCE[0]}")"

if [[ "$agendar" -eq 1 ]]; then
    [[ "$horario" =~ ^([01][0-9]|2[0-3]):[0-5][0-9]$ ]] || erro_sai "Horario invalido: $horario (use HH:MM)"
    hora="${horario%%:*}"
    minuto="${horario##*:}"

    case "$SO" in
        linux|osx)
            requer_comando crontab 'crontab nao encontrado no PATH; nao e possivel agendar automaticamente neste sistema.'
            linha_cron="$minuto $hora * * * $script_path --manter $manter"
            cron_atual="$(crontab -l 2>/dev/null || true)"
            if [[ -n "$cron_atual" ]] && grep -qF "$script_path" <<<"$cron_atual"; then
                aviso 'Ja existe uma entrada de backup agendada para este script; substituindo.'
                # "|| true": se a entrada removida era a unica linha do crontab,
                # o grep -v nao encontra nenhuma linha remanescente e sai com
                # status 1 (nao e um erro aqui, so significa "sobrou vazio").
                cron_atual="$(grep -vF "$script_path" <<<"$cron_atual" || true)"
            fi
            { [[ -n "$cron_atual" ]] && printf '%s\n' "$cron_atual"; printf '%s\n' "$linha_cron"; } | crontab -
            ok "Backup diario agendado para $horario via crontab do usuario."
            ;;
        windows)
            requer_comando schtasks.exe 'schtasks.exe nao encontrado no PATH.'
            bash_exec="$(command -v bash)"
            tr_valor="\"$bash_exec\" \"$script_path\" --manter $manter"
            schtasks.exe /Create /TN "FinancasLocal - Backup diario" /TR "$tr_valor" /SC DAILY /ST "$horario" /F >/dev/null
            ok "Backup diario agendado para $horario (Agendador de Tarefas > 'FinancasLocal - Backup diario')."
            ;;
    esac
    exit 0
fi

garantir_pasta "$PastaBackups"
arquivo="$PastaBackups/FinancasLocal_$(date +%Y%m%d_%H%M%S).db"

etapa "Fazendo backup em $arquivo"
[[ -f "$banco" ]] || erro_sai "Banco nao encontrado em $banco. Rode antes o 2-criar-banco.sh."
sqlite3 "$banco" ".backup '$arquivo'"
[[ "$(sqlite3 "$arquivo" 'PRAGMA integrity_check;')" == 'ok' ]] || erro_sai 'Verificacao de integridade do backup falhou.'
ok 'backup concluido e verificado'

mapfile -t antigos < <(ls -t "$PastaBackups"/FinancasLocal_*.db 2>/dev/null | tail -n +"$((manter + 1))")
if [[ "${#antigos[@]}" -gt 0 ]]; then
    rm -f "${antigos[@]}"
    ok "${#antigos[@]} backup(s) antigo(s) removido(s)"
fi
