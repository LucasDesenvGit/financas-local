# Instalador do Windows (.msi)

O `.msi` que instala o site, o gerenciador e o serviço no Windows, gerado com o WiX 5 pela CI ([ADR 0029](../docs/adr/0029-instalador-msi-com-wix-gerado-pela-ci.md)). O que ele instala e onde está em [docs/ARQUITETURA.md, "Instalador do Windows"](../docs/ARQUITETURA.md#instalador-do-windows--instalador); as opções para quem usa, em [docs/INSTALADOR-E-GERENCIADOR.md](../docs/INSTALADOR-E-GERENCIADOR.md).

| Arquivo | O que faz |
|---|---|
| `FinancasLocal.wxs` | o pacote: arquivos, serviço `FinancasLocal`, telas e atalhos |
| `configurar.ps1` | a ação personalizada que roda na instalação: pasta de dados protegida, certificado HTTPS, migração da instalação pelos scripts |
| `gerar-msi.ps1` | compila e monta o `.msi` (`-Revisao N` para a versão de desenvolvimento) |
| `imagens/` | as figuras das telas do instalador e o `gerar-imagens.ps1` que as faz |

**Testar uma mudança:** o `.msi` não é gerado no PR, só no merge para o `develop` ([ADR 0050](../docs/adr/0050-msi-gerado-no-merge-e-nao-no-pr.md)). Mexeu nesta pasta? Rode à mão o workflow "Versão (instalador .msi)" na branch e diga no PR que rodou.

**Cuidado:** serviço, certificado e pastas de dados também existem no `scripts/windows/instalar-windows.ps1`. Mudou um, confira o outro. O `configurar.ps1` tem as mesmas restrições do Windows PowerShell 5.1 (ASCII, sem `??`) e usa do `scripts/windows/comum.ps1` só o `Confiar-No-Certificado` (o `gerar-msi.ps1` leva o arquivo junto, para `instalador\`).

## A ação personalizada

- **Ação personalizada** (`configurar.ps1`, como SYSTEM, sem janela, com a saída no log do `msiexec`):
  - **na instalação, no reparo e na atualização**, antes de registrar o serviço:
    1. cria e protege a pasta de dados;
    2. **migra a instalação pelos scripts**, quando existe o serviço `FinancasLocal` apontando para outra pasta: para o serviço, copia o banco e as chaves (os `key-*.xml` antigos e o `segredos.chave` do Go, só se a pasta nova ainda não tem a dela; o `sessao-go.chave` não vai), sem apagar a pasta antiga, deixa uma nota e remove o serviço antigo, que tem o mesmo nome. Se o banco da instalação antiga não for achado (o caminho do `appsettings.Production.json` dela), para sem mudar nada;
    3. gera ou reaproveita o certificado (SAN `localhost`/`127.0.0.1`/`::1`) pelo `financas-go.exe preparar-https` (que também grava o `configuracao.json`) e o põe nas Autoridades Raiz da máquina. Reaproveita o `.pfx` que abre com a senha e vale por mais de 30 dias; a CI confere, na atualização, que ele foi reaproveitado;
  - **só na desinstalação de verdade** (não na troca de versão): tira o certificado. Com `APAGARDADOS=1`, apaga também os dados.

## Publicar e a versão de desenvolvimento

Os passos estão no [CONTRIBUINDO.md, "Publicar uma versão"](../CONTRIBUINDO.md#publicar-uma-versão). O workflow "Versão (instalador .msi)" publica o `.msi` e o `SHA256SUMS.txt` no Release da tag e, a cada push no `develop`, depois de todos os testes, recria o pré-lançamento `desenvolvimento` (`gerar-msi.ps1 -Revisao N`: `X.Y.Z-dev.N` no site, `X.Y.Z.N` no `.msi`). O detalhe de cada passo está nos comentários do `versao.yml`, do `gerar-msi.ps1` e do `FinancasLocal.wxs`.
