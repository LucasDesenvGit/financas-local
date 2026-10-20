#!/usr/bin/env bash
# Repasse temporario (#154): o backup mudou para scripts/geral/4-backup.sh.
# Fica aqui so para o backup diario agendado antes da mudanca (crontab ou
# Agendador de Tarefas, que guardam este caminho) continuar funcionando.
# Para trocar o agendamento pelo caminho novo: scripts/geral/4-backup.sh --agendar
# Pode ser apagado numa versao futura.
exec bash "$(dirname "${BASH_SOURCE[0]}")/geral/4-backup.sh" "$@"
