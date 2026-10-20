#!/usr/bin/env bash
# Repasse temporario (#154): este script mudou para scripts/linux/instalar-gerenciador.sh.
# Fica aqui so para o comando que o gerenciador do Linux antigo mostra
# ("git pull && scripts/3-publicar-atualizar.sh && scripts/instalar-gerenciador.sh")
# continuar funcionando na primeira atualizacao depois da mudanca.
# Pode ser apagado numa versao futura.
echo "Aviso: use scripts/linux/instalar-gerenciador.sh (este caminho antigo sera removido)." >&2
exec bash "$(dirname "${BASH_SOURCE[0]}")/linux/instalar-gerenciador.sh" "$@"
