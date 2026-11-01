#!/usr/bin/env bash
# =====================================================================
# Hook PreToolUse (Bash) dos agentes do projeto (.claude/agents/, #560):
# barra o que o agente nunca faz sozinho, porque depende de um "pode"
# de quem pediu, dado para aquele PR:
#   - gh pr merge;
#   - git push com --force, -f ou --force-with-lease.
# O resto ("não faça commit sem pedirem") depende do pedido, que o hook
# não lê: continua no texto do agente.
#
# Sai com 2 para barrar: o texto da saída de erro volta para o agente
# (documentação: hooks, PreToolUse). Confere o JSON de entrada inteiro,
# sem jq (Git Bash no Windows e qualquer Linux): um "gh pr merge" dentro
# de um echo também é barrado, de propósito.
# =====================================================================
entrada="$(cat 2>/dev/null || true)"

if grep -Eq 'gh[[:space:]]+pr[[:space:]]+merge' <<<"$entrada"; then
    echo "Barrado pelo hook do agente: merge de PR só com o \"pode\" de quem pediu, dado para aquele PR. Abra o PR, acompanhe a CI e devolva o link." >&2
    exit 2
fi
if grep -Eq 'git[[:space:]]+push[^|;&]*([[:space:]]-f([[:space:]"]|$)|--force)' <<<"$entrada"; then
    echo "Barrado pelo hook do agente: git push forçado reescreve o histórico de outra pessoa. Peça a quem chamou." >&2
    exit 2
fi
exit 0
