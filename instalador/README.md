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

## O que o pacote faz

- **Onde fica:** o programa em `%ProgramFiles%\FinancasLocal`; os dados em `%ProgramData%\FinancasLocal\dados` (`sql`, `chaves`, `backups`, `certificado`, `configuracao.json`), só para SYSTEM e Administradores (por SID). O `appsettings.Production.json` do pacote tem só `Dados:Pasta`: o site deriva o banco e as chaves dessa pasta e lê o `configuracao.json`, que guarda o certificado e a senha dele.
- **Serviço:** `FinancasLocal`, início automático, reinício em caso de falha, conta LocalSystem (o porquê está na ADR 0029). O executável do serviço é o `financas-go.exe -aplicar-esquema -rotinas-de-partida`, que lê o `appsettings.json` junto do `appsettings.Production.json` e do `configuracao.json`.
- **Ação personalizada** (`configurar.ps1`, como SYSTEM, sem janela, com a saída no log do `msiexec`):
  - **na instalação, no reparo e na atualização**, antes de registrar o serviço:
    1. cria e protege a pasta de dados;
    2. **migra a instalação pelos scripts**, quando existe o serviço `FinancasLocal` apontando para outra pasta: para o serviço, copia o banco e as chaves (os `key-*.xml` antigos e o `segredos.chave` do Go, só se a pasta nova ainda não tem a dela; o `sessao-go.chave` não vai), sem apagar a pasta antiga, deixa uma nota e remove o serviço antigo, que tem o mesmo nome. Se o banco da instalação antiga não for achado (o caminho do `appsettings.Production.json` dela), para sem mudar nada;
    3. gera ou reaproveita o certificado (SAN `localhost`/`127.0.0.1`/`::1`) pelo `financas-go.exe preparar-https` (que também grava o `configuracao.json`) e o põe nas Autoridades Raiz da máquina. Reaproveita o `.pfx` que abre com a senha e vale por mais de 30 dias; a CI confere, na atualização, que ele foi reaproveitado;
  - **só na desinstalação de verdade** (não na troca de versão): tira o certificado. Com `APAGARDADOS=1`, apaga também os dados.
- **Telas:** boas-vindas, pasta de instalação e progresso. No fim, uma caixa marcada para abrir o gerenciador; com o Firefox instalado, o aviso de que ele não usa os certificados do Windows por padrão ([Mozilla](https://wiki.mozilla.org/CA/AddRootToFirefox)) e como aceitar, como os scripts desde a #300 (só com interface, não no `/qn`). Sem o WebView2, a tela final manda para a página da Microsoft ([README do gerenciador](../src/gerenciador-go/README.md#instalação)).
- **Versão mínima do Windows:** a compilação 17763 (Windows 10 1809 / Server 2019), conferida no `Launch`. Ela vem do registro (`CurrentBuildNumber`), porque o `VersionNT` do Windows Installer responde 603 no Windows 8.1, no 10 e no 11 ([valores](https://learn.microsoft.com/windows/win32/msi/operating-system-property-values)); a comparação com um número é numérica ([sintaxe das condições](https://learn.microsoft.com/windows/win32/msi/conditional-statement-syntax)). O próprio Go pede só o [Windows 10 ou o Server 2016](https://go.dev/wiki/MinimumRequirements); quem limita é o `.pfx` do Go (`pkcs12.Modern2023`).

## Publicar uma versão

Os passos estão no [CONTRIBUINDO.md, "Publicar uma versão"](../CONTRIBUINDO.md#publicar-uma-versão): a versão já vem calculada dos merges do `develop` ([ADR 0070](../docs/adr/0070-versao-calculada-no-build-e-changelog-um-arquivo-por-pr.md); `go run ./cmd/versao`, em `src/backend`); o `scripts/dev/lancar-versao.sh X.Y.Z`, na `release/X.Y.Z`, junta `changelog.d/` numa seção do `CHANGELOG.md`, faz-se o merge em `main` e cria-se a tag `vX.Y.Z`, igual ao número calculado no commit da release. O workflow "Versão (instalador .msi)" publica o `.msi` e o `SHA256SUMS.txt` no Release. Para testar um PR, o `.msi` fica como artefato da execução manual do mesmo workflow.

**Título e notas do Release** (#187): na tag, o workflow cria ou edita o Release com o título "Finanças Local X.Y.Z" e as notas começando por "Como instalar ou atualizar" (o `.msi`, o aviso do Windows sobre instalador não assinado e o link para a instalação pelo código) e, em "O que mudou", a seção da versão no `CHANGELOG.md` sem o cabeçalho `## [X.Y.Z] — data`, que repetiria o título. Não é pré-lançamento nem rascunho e fica como o mais recente, que é o que o gerenciador consulta.

**Sem assinatura:** o Windows mostra "O Windows protegeu o computador"; passa-se por **Mais informações › Executar assim mesmo**.

## Versão de desenvolvimento

- **Rótulo:** fora da tag, `gerar-msi.ps1 -Revisao N`, com N = número da execução do workflow. O site e o gerenciador mostram `X.Y.Z-dev.N`; o `.msi` tem a versão `X.Y.Z.N`, mas o Windows Installer compara só `X.Y.Z` (o N só identifica o pacote).
- **A mesma versão substitui** (`AllowSameVersionUpgrades`): a de desenvolvimento e a do Release com o mesmo `X.Y.Z` trocam uma pela outra, sem ficarem as duas em "Aplicativos instalados". A CI testa a troca (desenvolvimento → Release da mesma versão) antes de testar a atualização para a versão seguinte.
- **Pré-lançamento `desenvolvimento`:** a cada push no `develop`, depois de todos os testes (instalar, trocar, atualizar, desinstalar, migrar dos scripts), o workflow apaga e recria o pré-lançamento com a tag `desenvolvimento`, apontando para o commit. O arquivo tem nome fixo, `FinancasLocal-desenvolvimento.msi`, com o `SHA256SUMS.txt`, então o link não muda.
- **Instalação pelos scripts:** o gerenciador não instala o `.msi` sozinho; mostra o caminho (rodar os scripts de novo, ou o botão "Baixar o instalador .msi", que abre a página da versão mais recente). O `.msi` migra a instalação (item 2 da ação personalizada).

## A CI do instalador

- **O WiX** (#446): o passo "WiX" do `versao.yml` baixa o `wix-cli-x64.msi` da versão fixada (`WIX_VERSAO`), confere o SHA-256 (`WIX_SHA256`, o mesmo do `wix-cli-x64.metadata.json` que o WiX publica junto), instala com `msiexec /qn` e põe a pasta do `wix.exe` (a variável `WIX5` que o `.msi` grava) no PATH dos passos seguintes. Trocar de versão pede os dois valores novos.
- **Medidas:** a cada execução, o log escreve o tamanho do `.msi` e da pasta instalada e a memória do site (linhas `TAMANHO` e `MEMORIA`). O `.msi` caiu de 57 MB para 53 MB com o gerenciador em Go (#277); os números de cada versão foram no épico #70. São do runner do Windows, não de um computador de uso: servem para comparar versões.
- **Por que dois workflows instalam pelos scripts** (avaliado na #204, sem mudança): o `teste-instalacao-windows.yml` instala, atualiza, para e sobe de novo pelos scripts; o job "migrar da instalação pelos scripts" do `versao.yml` instala pelos scripts e migra para o `.msi`. Juntar exigiria levar a instalação de um runner para outro; os gatilhos são diferentes e os dois rodam em paralelo. O "Versão" também não reaproveita o build da CI, que compila em Linux e não gera o pacote para `win-x64`.

## Histórico

- **O pacote "só Go"** (#277, parte 4): o `gerar-msi.ps1 -SoGo` montou o pacote sem a API em .NET, com o serviço no `financas-go.exe`, e a CI o instalava sobre os dados mantidos do pacote normal, conferindo que o usuário continuava lá, que o login do Go entrava com a mesma senha e que o `/api/go/saude` respondia. Ficou desligado até a inversão (#85); com a saída do .NET (#86), é o único pacote, e a opção saiu.
- **Até a #86**, o pacote levava a API em .NET self-contained (com o runtime do .NET, do ASP.NET e do Windows Desktop), e o workflow precisava do `setup-dotnet` para o `dotnet publish`; o WiX deixou de precisar dele na #446.
