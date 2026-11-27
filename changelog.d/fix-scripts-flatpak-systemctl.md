### Correções
- **Instalação no Linux pelo terminal de um app em Flatpak** (#789). Rodando os scripts no terminal do VS Code em Flatpak (como no Bazzite), o serviço do site agora é registrado e sobe no sistema, e parar e instalar o gerenciador enxergam o serviço.
  - Técnico: sem `systemctl` no sandbox, o `comum.sh` chama o do host com `flatpak-spawn --host`.
