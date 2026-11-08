# Instalador do Windows (.msi)

O `.msi` que instala o site, o gerenciador e o serviço no Windows, gerado com o WiX 5 pela CI ([ADR 0029](../docs/adr/0029-instalador-msi-com-wix-gerado-pela-ci.md)). O que ele instala e onde está em [docs/ARQUITETURA.md, "Instalador do Windows"](../docs/ARQUITETURA.md#instalador-do-windows--instalador); as opções para quem usa, em [docs/INSTALADOR-E-GERENCIADOR.md](../docs/INSTALADOR-E-GERENCIADOR.md).

| Arquivo | O que faz |
|---|---|
| `FinancasLocal.wxs` | o pacote: arquivos, serviço `FinancasLocal`, telas e atalhos |
| `configurar.ps1` | a ação personalizada que roda na instalação: pasta de dados protegida, certificado HTTPS, migração da instalação pelos scripts |
| `gerar-msi.ps1` | compila e monta o `.msi` (`-Revisao N` para a versão de desenvolvimento) |
| `imagens/` | as figuras das telas do instalador e o `gerar-imagens.ps1` que as faz |

**Testar uma mudança:** o `.msi` não é gerado no PR, só no merge para o `develop` ([ADR 0050](../docs/adr/0050-msi-gerado-no-merge-e-nao-no-pr.md)). Mexeu nesta pasta? Rode à mão o workflow "Versão (instalador .msi)" na branch e diga no PR que rodou.

**Cuidado:** serviço, certificado e pastas de dados também existem no `scripts/windows/instalar-windows.ps1`. Mudou um, confira o outro.
