# 0029. Instalador .msi para Windows, com WiX, gerado e testado pela CI

- **Status:** Aceita (em parte substituída pela [0039](0039-versao-sobe-a-cada-pr-pelo-tipo-do-commit.md), que a [0070](0070-versao-calculada-no-build-e-changelog-um-arquivo-por-pr.md) substituiu: a versão não é escolhida na hora de publicar, é calculada no build a cada merge)
- **Data:** 2026-10-17 (issue #119)
- **Relacionadas:** substitui em parte a [0012](0012-instalador-do-windows-em-powershell.md) (instalador em PowerShell); [0003](0003-esquema-por-scripts-sql-idempotentes.md) (a API aplica o esquema); [0028](0028-gerenciador-do-windows-e-atualizacao-pelo-github.md) (gerenciador); [0005](0005-segredos-criptografados-com-data-protection.md) (chaves); [0050](0050-msi-gerado-no-merge-e-nao-no-pr.md) (o `.msi` só no merge)

**Os passos** (publicar uma versão, o que a ação personalizada faz, a versão de desenvolvimento, as notas do Release, o WiX na CI) estão no [README do instalador](../../instalador/README.md). Esta ADR guarda a decisão, as alternativas e o custo.

## Contexto

A [ADR 0012](0012-instalador-do-windows-em-powershell.md) instala no Windows com `instalar-windows.cmd` e o `instalar-windows.ps1`, que no computador do usuário baixam as ferramentas, compilam tudo (minutos e mais de 1 GB de ferramentas) e registram o serviço. Atualizar é repetir isso. A 0012 descartou um instalador empacotado porque ele "exige empacotar e assinar um executável a cada versão".

A issue #119 pediu um `.msi` de "Avançar, Avançar, Concluir" e um gerenciador. Com a CI gerando o pacote a cada versão, o custo de empacotar deixa de ser manual.

## Alternativas consideradas

- **Continuar só com o PowerShell (ADR 0012).** Nada novo, mas compila na máquina de quem só quer usar, e atualizar exige terminal ou script como administrador.
- **Inno Setup ou NSIS** (`setup.exe`). Script simples e interface fácil de personalizar, mas não é MSI (sem atualização e remoção padronizadas pelo Windows Installer, nem implantação por política), e serviço e certificado viram script do mesmo jeito.
- **MSIX.** Atualização moderna, mas não instala serviço do Windows de forma simples, exige assinatura e isola o app do resto do sistema.
- **WiX Toolset 5** (código aberto). MSI padrão, com serviço (`ServiceInstall`/`ServiceControl`), atualização (`MajorUpgrade`), atalhos e interface em pt-BR; compila no runner Windows da CI; o elemento `Files` coleta a pasta publicada. Contra: o que o MSI não faz sozinho (certificado, permissões, migração) fica numa ação personalizada em PowerShell.

## Decisão

O `.msi` é gerado por `instalador/gerar-msi.ps1` a partir de `instalador/FinancasLocal.wxs` (WiX 5.0.2), pela CI.

- **Tudo na mesma pasta, sem nada instalado antes:** o site e o gerenciador vão prontos para `win-x64`; o computador não precisa de .NET, Node, Git, bash nem `sqlite3`. A API cria e atualiza o banco ao iniciar (ADR 0003). Desde a #277, o gerenciador é o em Go (ADR 0052).
- **A CI gera, testa e publica:** o workflow "Versão (instalador .msi)" testa o que o usuário vai fazer (instalar do zero, atualizar, desinstalar mantendo os dados, migrar uma instalação feita pelos scripts) e, na tag `vX.Y.Z`, publica o `.msi` e o `SHA256SUMS.txt` no Release. A versão é calculada no build (ADR 0070), e a tag tem de ser igual à do commit da release. Desde a [0050](0050-msi-gerado-no-merge-e-nao-no-pr.md), ele roda no merge para o `develop`, não no PR.
- **Programa em `%ProgramFiles%\FinancasLocal`; dados em `%ProgramData%\FinancasLocal\dados`**, só para SYSTEM e Administradores. Atualizar e desinstalar não mexem nos dados.
- **Serviço `FinancasLocal`, conta LocalSystem**, como na instalação pelos scripts: as chaves dos segredos usam o DPAPI da conta do serviço, e com outra conta (ex.: `NT SERVICE\FinancasLocal`) a migração perderia o Client Secret da Pluggy.
- **Ação personalizada** (`instalador/configurar.ps1`, como SYSTEM): protege a pasta de dados, **migra a instalação pelos scripts** (sem apagar a pasta antiga) e gera ou reaproveita o certificado HTTPS, posto nas Autoridades Raiz da máquina, como na 0012. Na desinstalação de verdade, tira o certificado; os dados só saem com `APAGARDADOS=1`.
- **Interface:** boas-vindas, pasta de instalação e progresso, sem tela de licença (o projeto não tem uma).

## Consequências

- **Positivas:** instalação em dois cliques, sem compilar nada no computador; atualização pelo gerenciador ou abrindo o `.msi` novo; o Windows lista o programa em "Aplicativos instalados"; a CI testa o caminho de quem usa.
- **Negativas:**
  - **sem assinatura de código** (em aberto na #119), o Windows mostra "Editor desconhecido" ao abrir o `.msi`. Assinar exige um certificado de código pago;
  - **dois caminhos de instalação no Windows:** mudança no certificado, no serviço ou na pasta de dados precisa ser feita no `instalar-windows.ps1` e no `configurar.ps1`, que tem as mesmas restrições do PowerShell 5.1 (ASCII, sem `??`);
  - **o pacote fica grande**, na casa de dezenas de MB (era o runtime do .NET dentro; com o Go, caiu, veja o README);
  - **migração automática:** a pasta antiga fica, mas o serviço antigo é removido. Para voltar, rode o `instalar-windows.cmd` antigo de novo. Se o banco da instalação antiga não for achado, a instalação **para sem mudar nada**, em vez de começar com um banco vazio;
  - **só Windows x64.** No Linux continuam os scripts.
- **Na prática:** mudou serviço, certificado ou pastas? Ajuste também o `instalar-windows.ps1` (e vice-versa). Publicar uma versão e testar uma mudança: [README do instalador](../../instalador/README.md).

## Atualização de 2026-10-18: versão de desenvolvimento

Pedido do usuário: deixar o `.msi` do `develop` à mão e garantir que quem já usa atualize sem complicação.

- **Decisão:** fora da tag, o pacote é de desenvolvimento (`X.Y.Z-dev.N` no site e no gerenciador). A mesma versão substitui a outra (`MajorUpgrade AllowSameVersionUpgrades="yes"`), e a cada push no `develop`, depois de todos os testes, o workflow recria o pré-lançamento `desenvolvimento`, com um link fixo. Pré-lançamento nunca é o "latest", então quem está no Release não recebe aviso dele (ajustado pela ADR 0040: com uma versão de desenvolvimento instalada, o gerenciador consulta o pré-lançamento).
- **Alternativas:** guardar o `.msi` no repositório (~50 MB por versão no histórico, e velho a cada merge); só o artefato do workflow (some em 14 dias, pede login e não tem link fixo); versão de desenvolvimento como `X.Y.(Z+1)` (o Release seguinte teria a mesma versão do pacote de desenvolvimento e prometeria um número que talvez nem saia; *revisto pela ADR 0039*).
- **Negativas:** mais tempo de CI a cada push no `develop` (dezenas de minutos num runner Windows); **rebaixamento silencioso**: instalar o Release `X.Y.Z` por cima do desenvolvimento `X.Y.Z.N` "volta" funcionalidades sem aviso (o banco fica com as colunas novas, o que é seguro, porque o esquema só acrescenta, ADR 0003); entre apagar e recriar a tag `desenvolvimento`, por alguns segundos, o link dá 404.

## Atualização de 2026-10-20: título e notas do Release sempre acertados (issue #187)

O Release criado à mão antes da tag ficava com o título e as notas da mão (as versões 1.0.0 e 2.0.0 saíram assim, sem o `.msi`). **Decisão:** na tag, o workflow cria **ou edita** o Release, sempre com o título "Finanças Local X.Y.Z" e as notas tiradas do `CHANGELOG.md`. **Custo:** quem editar as notas de um Release à mão e depois recriar a tag perde a edição; é no `CHANGELOG.md` que se corrige.

## Atualização de 2026-11-02: o `.msi` preparado para o Go (issue #277)

- **Certificado e chaves pelo Go:** o certificado passa a ser gerado pelo `financas-go.exe preparar-https` (#84, #775), como nos scripts, e a migração dos scripts copia também a chave dos segredos do Go (`segredos.chave`), pela mesma regra das chaves antigas. Vale porque o serviço antigo e o novo rodam na mesma conta, dona do DPAPI.
- **Versão mínima do Windows:** o `.msi` e o `instalar-windows.ps1` recusam um Windows abaixo da compilação **17763** (Windows 10 1809 / Server 2019), com uma mensagem clara, em vez de um site que não abre. O motivo é o certificado: o Go grava o `.pfx` com AES e SHA-256, que o Windows lê [a partir do Server 2019](https://pkg.go.dev/software.sslmate.com/src/go-pkcs12#pkg-variables).
- **Firefox:** a tela final avisa que o Firefox não usa os certificados do Windows por padrão e diz como aceitar. O instalador **não muda o Firefox sozinho**: ligar `security.enterprise_roots.enabled` faz ele confiar em todos os certificados do sistema, inclusive os de empresa, e essa escolha é de quem usa.
- **Ficaram de fora** (*julgamento de quem fez a #277, não confirmado com o time*; cada um vira issue quando alguém precisar): **assinatura do `.msi`** (continua sem; o README ensina a passar pelo aviso do Windows); **Windows ARM** (compila barato, mas nunca foi testado e a CI não tem runner ARM com Windows); **pacote pronto no Linux** (o gerenciador precisa de cgo e das bibliotecas da própria distribuição, o que pede um pacote por distribuição).
- **Consequência negativa:** quem está num Windows 10 anterior ao 1809 (sem suporte da Microsoft desde 2020) não consegue mais instalar nem atualizar; a versão já instalada continua funcionando enquanto o certificado atual valer.

## Atualização de 2026-11-02: o WiX sem o SDK do .NET (issue #446)

**Decisão (do usuário, 02/11):** o WiX 5.0.2 passa a vir do `.msi` do próprio CLI (`wix-cli-x64.msi`, no GitHub Releases do WiX), e não do `dotnet tool install`. Ele instala a compilação **net472** do `wix.exe`, que roda no .NET Framework 4.7.2 do próprio Windows, sem o SDK do .NET moderno, e já traz as extensões `UI` e `Util` (conferido no código-fonte: [`wix-cli.wixproj`](https://github.com/wixtoolset/wix/blob/v5.0.2/src/setup/wix-cli/wix-cli.wixproj), [`Package.wxs`](https://github.com/wixtoolset/wix/blob/v5.0.2/src/setup/wix-cli/Package.wxs)).

- **Alternativas descartadas:** manter o `dotnet tool` (o SDK do .NET ficaria na CI só por causa do WiX); outra ferramenta de `.msi` (reescrever o `.wxs`, com dependência nova e sem ganho).
- **Ficamos no WiX 5.0.2.** Subir para o 6 ou o 7 é outra decisão. *Inferido, não conferido:* desde o WiX 6, o projeto pede uma taxa de manutenção (OSMF) a quem ganha dinheiro com ele.
- **Consequência negativa:** trocar a versão do WiX pede atualizar o SHA-256 fixado à mão.
