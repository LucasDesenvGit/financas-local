#!/usr/bin/env bash
# =====================================================================
# Hook UserPromptSubmit do Claude Code (#280): lembra de oferecer a skill
# trabalho-longo (.claude/skills/trabalho-longo) quando o trabalho tem cara
# de passar do limite de uso:
#   - o pedido traz link de issue/PR do GitHub ou fala de epico;
#   - a sessao ja e longa (transcricao acima de 8 MB, e de novo a cada 25 MB).
# O texto impresso vira contexto para o Claude (documentacao: hooks,
# UserPromptSubmit). Avisa uma vez por motivo e por sessao.
#
# Nunca bloqueia o pedido: sai sempre com 0. Sem dependencia nova (bash,
# grep, sed, wc: Git Bash no Windows e qualquer Linux).
# Desligar: FINANCAS_SEM_LEMBRETE_TRABALHO_LONGO=1
# =====================================================================
[[ "${FINANCAS_SEM_LEMBRETE_TRABALHO_LONGO:-}" == 1 ]] && exit 0

entrada="$(cat 2>/dev/null || true)"
[[ -n "$entrada" ]] || exit 0

campo() {  # valor de um campo de texto do JSON de entrada (sem jq)
    sed -n "s/.*\"$1\"[[:space:]]*:[[:space:]]*\"\([^\"]*\)\".*/\1/p" <<<"$entrada" | head -n1
}
sessao="$(campo session_id | tr -cd 'A-Za-z0-9-')"
transcricao="$(campo transcript_path | sed 's/\\\\/\\/g')"
[[ -n "$sessao" ]] || exit 0

marca="${TMPDIR:-/tmp}/financas-trabalho-longo-$sessao"
avisado_pedido=0; avisado_tamanho=0
if [[ -f "$marca" ]]; then
    read -r avisado_pedido avisado_tamanho < "$marca" 2>/dev/null || true
fi
avisado_pedido="${avisado_pedido:-0}"; avisado_tamanho="${avisado_tamanho:-0}"

tamanho=0
if [[ -n "$transcricao" && -f "$transcricao" ]]; then
    tamanho=$(( $(wc -c < "$transcricao" 2>/dev/null || echo 0) / 1048576 ))
fi

motivo=""
if [[ "$avisado_pedido" == 0 ]] &&
   grep -qiE 'github\.com/[^ ]+/(issues|pull)/[0-9]+|[ée]pico|(v[aá]rias|todas as) (issues|tasks|hist[oó]rias)' <<<"$entrada"; then
    motivo="o pedido cita issue/PR do GitHub ou um epico"
    avisado_pedido=1
fi
if { [[ "$avisado_tamanho" == 0 && "$tamanho" -ge 8 ]] || [[ "$avisado_tamanho" -gt 0 && "$tamanho" -ge $(( avisado_tamanho + 25 )) ]]; }; then
    motivo="${motivo:+$motivo; }a sessao ja tem ${tamanho} MB de transcricao (sessao longa)"
    avisado_tamanho="$tamanho"
fi
[[ -n "$motivo" ]] || exit 0

printf '%s %s\n' "$avisado_pedido" "$avisado_tamanho" > "$marca" 2>/dev/null || true
cat <<EOF
Lembrete do projeto (hook trabalho-longo, #280): $motivo. Pela regra do CLAUDE.md ("Pedido grande demais para uma vez so"), este e um momento de oferecer em uma frase a skill trabalho-longo, que deixa agendado o "continua o plano" para o trabalho seguir depois do limite de uso. Se ja existe agendamento nesta sessao (CronList) ou a pessoa ja recusou, nao e preciso oferecer de novo.
EOF
exit 0
