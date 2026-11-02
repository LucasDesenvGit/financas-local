# Instalar a partir do código

Para quem usa **Linux**, quer mexer no código ou prefere os scripts ao instalador. No Windows, para só usar, o caminho mais simples é o [instalador `.msi`](../README.md#instalar-no-windows).

## Instalar

**Windows, com dois cliques:** dê dois cliques em `instalar-windows.cmd`, na raiz do projeto. Ele pede permissão de Administrador e roda `scripts/windows/instalar-windows.ps1`, que faz as três etapas abaixo usando só o PowerShell que já vem no Windows — não precisa instalar Git, bash, Python nem nada antes. No fim, abre o site no navegador. Pode rodar de novo para atualizar: antes, o banco é copiado para `backups/`, e se a versão nova não subir a anterior volta sozinha. Se algo falhar, a janela fica aberta com o erro e a etapa em que parou, e a saída completa fica em `instaladores/instalacao-windows.log`.

**Linux (ou Windows pelo Git Bash), com um comando:**

```bash
cd financas-local/scripts/geral
./instalar.sh                # roda 1, 2 e 3 em sequência e para no primeiro erro
```

**Etapa por etapa (bash).** Abra um terminal com bash (no Linux, o do sistema; no Windows, o **Git Bash**, que vem com o [Git for Windows](https://git-scm.com/download/win), ou o WSL). No Windows, abra o Git Bash **como Administrador** para o `3-publicar-atualizar.sh`, que registra o serviço. O backup (`4-backup.sh`) continua precisando de bash no Windows.

```bash
cd financas-local/scripts/geral
./1-preparar-ambiente.sh     # .NET 10 SDK, Go, Node.js e sqlite3 (poucos minutos)
./2-criar-banco.sh           # cria o banco SQLite com a estrutura completa
./3-publicar-atualizar.sh    # compila e publica como serviço do sistema
```

Depois, abra **https://localhost:5443**. No primeiro acesso você cria o usuário e a senha (mínimo de 8 caracteres).

### Gerenciador do Finanças Local

**Windows**, sem precisar de administrador:

```powershell
powershell -ExecutionPolicy Bypass -File scripts\windows\instalar-gerenciador.ps1
```

Ele publica o gerenciador em `publicado\gerenciador` e cria os atalhos no Menu Iniciar e na área de trabalho (`-SemAtalhoNaAreaDeTrabalho` para não criar este último). O gerenciador acha o site pelo serviço e mostra a situação e a versão, liga, desliga e copia o banco. Ele não baixa a versão nova, porque nessa instalação ela vem pelos scripts: diz como atualizar por eles e sugere passar para o `.msi`, que traz o banco e as chaves e, daí em diante, recebe a versão nova pelo gerenciador. Rode o mesmo comando de novo para atualizar o gerenciador.

**Linux:** o `instalar.sh` já instala o gerenciador no fim, sem `sudo` (#300). Para não instalar, use `scripts/geral/instalar.sh --sem-gerenciador`. Para instalar ou atualizar só ele:

```bash
scripts/linux/instalar-gerenciador.sh
```

- **Onde fica:** publicado em `publicado/gerenciador`, com o atalho **Gerenciador do Finanças Local** no menu de aplicativos e na área de trabalho (`--sem-atalho-na-area-de-trabalho` para não criar este último).
- **O que faz:** mostra se o site está no ar, liga, desliga e reinicia o serviço (sem senha: ele é do seu usuário), copia o banco e mostra o registro do serviço (**Ver o registro do serviço**, quando o site não sobe).
- **Versão nova:** mostra o comando para atualizar, pronto para copiar. Ele não acessa a internet, a não ser na instalação de desenvolvimento (clone na branch `develop`), em que o botão **Atualizar para a última do develop** traz o código do GitHub ([abaixo](#atualizar)).
- **Ao sair da conta:** o serviço do usuário para, e o site junto. Em **Mais opções**, **Manter o site no ar sem login** liga o *linger* do systemd (`loginctl enable-linger`).
- **Iniciar com o sistema:** deixa o ícone na bandeja e avisa se o site cair. No GNOME, a bandeja precisa da extensão AppIndicator; sem ela, a janela abre minimizada.
- **No terminal:** `publicado/gerenciador/Financas.Gerenciador.Linux --situacao` mostra a situação sem abrir a janela.

### De onde vem cada coisa

| Componente | Origem | Verificação |
|---|---|---|
| .NET 10 SDK (portátil em `ferramentas/dotnet`) | metadados oficiais `builds.dotnet.microsoft.com` | SHA-512 publicado nos metadados oficiais (não há verificação de assinatura) |
| Go (portátil em `ferramentas/go`, compila o back-end em Go que roda atrás da API: [ADR 0049](adr/0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md)) | índice oficial `go.dev/dl`, arquivo de `dl.google.com`; os módulos do `go.sum` vêm de `proxy.golang.org` | SHA-256 do índice oficial; os módulos, pelo `go.sum` do repositório |
| Node.js LTS (portátil em `ferramentas/node`, usado só para compilar o Angular) | `nodejs.org` | SHA-256 do `SHASUMS256.txt` oficial |
| `sqlite3` (CLI) | o do sistema, se houver; senão portátil em `ferramentas/sqlite`, baixado de `www.sqlite.org` (Linux arm64: gerenciador de pacotes) | SHA3-256 publicado na página oficial de download |

`1-preparar-ambiente.sh` confere antes as ferramentas básicas do terminal (`curl`, `tar`, `sha256sum`…, todas presentes no Git Bash), depois verifica cada um desses componentes, só baixa/instala o que ainda não estiver disponível e, no final, mostra a versão de cada um. Rode-o de novo a cada poucos meses para receber as correções de segurança do .NET.

## Atualizar

```bash
git pull
scripts/geral/instalar.sh     # no Windows, num Git Bash aberto como Administrador; no Linux, atualiza também o gerenciador
```

Use o `instalar.sh`, e não só o `3-publicar-atualizar.sh`: quando o `src/backend-go/go.mod` pede um Go mais novo, só a etapa 1 o baixa, e a publicação sozinha falha (ela compila com o Go de `ferramentas/go`, com `GOTOOLCHAIN=local`). O mesmo vale para o .NET. No Windows sem bash, rode o `instalar-windows.cmd` de novo, depois do `git pull`. Ele atualiza e, se a versão nova não subir, volta para a anterior sozinho. Em qualquer caso, a publicação para o serviço por alguns segundos, copia os arquivos novos e inicia o serviço de novo. Nenhuma atualização apaga dados.

**Instalação de desenvolvimento (clone na branch `develop`), pelo gerenciador** ([ADR 0040](adr/0040-atualizar-pelo-develop-so-na-instalacao-de-desenvolvimento.md)): o botão **Atualizar para a última do develop** faz o mesmo que `scripts/geral/atualizar.sh` (Linux) ou `scripts/windows/atualizar-windows.ps1` (Windows, num PowerShell como Administrador): confere que a pasta está no `develop` e sem alterações locais, traz o `develop` (só avança, nunca descarta nada) e reinstala. Com alteração local, outra branch ou commits locais que não estão no GitHub, recusa e diz o motivo. Fora do `develop` (numa tag, por exemplo), o botão não aparece.

### Instalado pelos scripts e quer passar para o instalador

No Windows, dá para passar desta instalação (feita com o `instalar-windows.cmd` ou o `3-publicar-atualizar.sh`) para o `.msi` a qualquer momento: baixe o `.msi` da [versão mais recente](https://github.com/LucasDesenvGit/financas-local/releases/latest) e abra. Ele encontra a instalação antiga, **copia o banco e as chaves** para `C:\ProgramData\FinancasLocal\dados` e troca o serviço antigo pelo dele. A pasta antiga **não é apagada**, e se o banco antigo não for achado, ele para sem mudar nada e o site antigo volta a rodar. O Client Secret da Pluggy continua funcionando. Daí em diante, a versão nova chega pelo gerenciador ([Instalador e gerenciador](INSTALADOR-E-GERENCIADOR.md)).

## Parar e desinstalar

O sistema instalado são **dois processos**: a API (.NET), rodando como serviço do sistema, e o back-end em Go (`financas-go`), que a API sobe como processo filho e que para junto com ela ([ADR 0049](adr/0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md)). O front-end (Angular) é publicado dentro da API e roda no navegador, e o banco (SQLite) é só o arquivo `dados/sql/FinancasLocal.db`, aberto pelos dois, sem serviço nem porta.

- **Parar no Windows:** dê dois cliques em `parar-windows.cmd`, na raiz (pede permissão de Administrador; não precisa de bash).
- **Parar no Linux/macOS:** `scripts/geral/parar.sh`.

Os dois param o serviço, a API rodando fora dele (`dotnet run`, Visual Studio) e o front de desenvolvimento (`npm start`), e avisam se algum programa ainda está com o banco aberto. Opções: `--desativar` / `-Desativar` (não sobe sozinho no próximo login/boot) e `--compiladores` / `-Compiladores` (encerra também os servidores de build do .NET). Para subir de novo, rode a publicação (`3-publicar-atualizar.sh` ou `instalar-windows.cmd`).

**Desinstalar:** pare com `parar-windows.cmd -Desativar` (Windows) ou `scripts/geral/parar.sh --desativar` (Linux) e apague a pasta do projeto. Os dados ficam em `dados/` e as cópias em `backups/`, dentro dela.

- **Windows:** remova também o serviço, num terminal aberto como Administrador: `sc.exe delete FinancasLocal`. O certificado do site fica nos confiáveis: pelo `instalar-windows.cmd`, nos do computador (`Cert:\LocalMachine\Root`, com o nome amigável `FinancasLocal`), que se apagam em `certlm.msc` › **Autoridades de Certificação Raiz Confiáveis**; pelo Git Bash, nos da sua conta, pelo `dotnet dev-certs https --trust`, que se apagam com `dotnet dev-certs https --clean`.
- **Linux:** apague a unidade do serviço, `~/.config/systemd/user/financaslocal.service`, e rode `systemctl --user daemon-reload`. Se ligou **Manter o site no ar sem login**, desligue com `loginctl disable-linger`. Com o gerenciador, apague também `~/.local/share/applications/financaslocal-gerenciador.desktop`, `~/.config/autostart/financaslocal-gerenciador.desktop` e o atalho da área de trabalho. O certificado que você pôs nos confiáveis (tabela em [O navegador alerta sobre o certificado](#o-navegador-alerta-sobre-o-certificado)) sai com o comando de remoção da sua distribuição.

## Migrar de uma instalação com SQL Server

Se você já usava o Finanças Local no Windows com IIS e SQL Server Express, use a ferramenta de migração para trazer os dados para o novo banco SQLite. **Não rode o `2-criar-banco.sh` antes**: os dados iniciais dele (categorias e configurações padrão) conflitam com os dados migrados. O destino precisa ser um banco vazio, só com a estrutura:

```bash
# 1. Pare o site antigo no IIS, para nada mudar no SQL Server durante a cópia.
# 2. Crie o banco vazio, só com a estrutura:
sqlite3 dados/sql/FinancasLocal.db < banco/01-estrutura.sql
# 3. Migre (a connection string antiga está no appsettings.Production.json da publicação do IIS):
dotnet run --project src/backend/Financas.MigrarSqlite -- \
  'Server=.\FINANCAS;Database=FinancasLocal;Integrated Security=True;Encrypt=True;TrustServerCertificate=True' \
  dados/sql/FinancasLocal.db
# 4. Publique normalmente:
scripts/geral/3-publicar-atualizar.sh
```

A ferramenta não altera o SQL Server (só lê dele) e confere a contagem de linhas de cada tabela no final. A pasta `dados\sql` da instalação antiga só é acessível por Administradores, então no Windows rode esses comandos num Git Bash aberto como Administrador.

É seguro rodar mesmo sem um SQL Server disponível: a ferramenta detecta que não há nada para migrar e termina sem erro. Como o Client Secret da Pluggy e o token da brapi foram criptografados com chaves da instalação antiga (que não podem ser lidas em outra máquina ou sistema operacional), a migração limpa esses dois campos — informe-os de novo em **Configurações** depois de migrar.

## Desenvolvimento (Visual Studio ou VS Code)

- Abra `FinancasLocal.sln`. O perfil *Desenvolvimento (Kestrel)* sobe a API em `https://localhost:7443`. Em desenvolvimento, as chaves ficam em `dados\chaves-dev`, então o Client Secret precisa ser informado de novo.
- **Back-end em Go:** a API sobe o Go como processo filho e repassa a ele as rotas de `Go:Prefixos` ([ADR 0049](adr/0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md)). Compile-o para a pasta de saída da API antes de subi-la, e de novo a cada mudança no Go; **sem o `financas-go` em `bin/Debug/net10.0/`, essas rotas respondem 503**:
  ```bash
  cd src/backend-go
  go build -o ../backend/Financas.Api/bin/Debug/net10.0/financas-go ./cmd/financas        # Linux
  go build -o ../backend/Financas.Api/bin/Debug/net10.0/financas-go.exe ./cmd/financas    # Windows
  ```
  O resto (rodar sozinho, testes, contrato) está no [README do Go](../src/backend-go/README.md).
- No front-end, rode `npm start` em `src\frontend`. O Angular abre em `http://localhost:4200` e redireciona `/api` para a API.
- Sem o .NET 10, o Go e o Node instalados, use as ferramentas portáteis do `1-preparar-ambiente.sh`, pondo-as na frente do PATH (troque o caminho pelo da sua pasta do projeto):
  `$env:PATH = "C:\caminho\financas-local\ferramentas\dotnet;C:\caminho\financas-local\ferramentas\go\bin;C:\caminho\financas-local\ferramentas\node;$env:PATH"`
- **Banco:** não há migrations. Toda a estrutura está em `banco/01-estrutura.sql`, que é idempotente. Para mudar o banco, veja [BANCO-DE-DADOS.md](BANCO-DE-DADOS.md).
- **Testes do front-end**, em `src/frontend`:
  - `npm test` e `npm run test:tipos`: Jest, sem navegador ([ADR 0013](adr/0013-testes-do-front-end-com-jest.md));
  - `npm run test:navegador`: compila o build de produção e abre cada tela num navegador de verdade (o Edge instalado), com a CSP da API e uma API falsa, sem back-end nem dado real. Falha com erro de console, exceção ou bloqueio de CSP ([ADR 0041](adr/0041-testes-no-navegador-com-playwright.md));
  - `FOTOS=painel,metas FOTOS_PASTA=<pasta> npx playwright test fotos`: prints das telas, em claro e escuro, no computador e no celular;
  - `npx playwright test --ui`: para ver passo a passo.
- Branches, commits, versão e checklist do PR: [CONTRIBUINDO.md](../CONTRIBUINDO.md). Arquitetura: [ARQUITETURA.md](ARQUITETURA.md). Decisões: [ADRs](adr/README.md).

### Estrutura

```
financas-local/
├── .claude/               skills, agentes e hooks do Claude Code (ver CONTRIBUINDO.md)
├── .github/               CI, templates de PR/issue, CODEOWNERS, Dependabot
├── banco/                 scripts SQL (estrutura, dados iniciais)
├── docs/                  arquitetura, guia do banco, esta página e ADRs (docs/adr)
├── instalador/            instalador .msi do Windows (WiX): FinancasLocal.wxs, configurar.ps1 e gerar-msi.ps1
├── scripts/
│   ├── geral/            o bash dos dois sistemas (no Windows, pelo Git Bash): instalar, publicar, banco, backup, parar
│   ├── windows/          o que só roda no Windows (instalar, parar e atualizar, o gerenciador e o configurar-github.ps1)
│   ├── linux/            o que só roda no Linux (gerenciador)
│   └── ci/               o que só a CI usa (verificadores, mutação, cobertura)
├── src/backend/           Financas.Api (C# / .NET 10 / EF Core) e Financas.MigrarSqlite (migração do SQL Server)
├── src/backend-go/        o back-end em Go, atrás da API (ADR 0049)
├── src/frontend/          financas-web (Angular)
├── src/gerenciador/       Gerenciador: núcleo comum, app do Windows (WinForms, .msi) e do Linux (Avalonia)
├── src/gerenciador-go/    o gerenciador em Go, que vai substituir os dois (ADR 0052)
├── dados/                 [gerado] banco (SQLite), chaves, logs — acesso restrito
├── backups/               [gerado] backups do banco
├── publicado/             [gerado] aplicação publicada
├── ferramentas/           [gerado] .NET SDK, Go, Node.js, sqlite3 (se preciso), caches NuGet/npm
└── instaladores/          [gerado] instaladores baixados (podem ser apagados)
```

## Problemas comuns (instalação pelos scripts)

| Sintoma | O que fazer |
|---|---|
| Windows: `Python was not found`, `sqlite3: command not found` ou `unzip: command not found` | Versão antiga dos scripts. Atualize o repositório e use o `instalar-windows.cmd`, que não depende de bash nem de Python |
| Windows: o `instalar-windows.cmd` falhou | Veja a linha `ERRO` na janela e o log completo em `instaladores/instalacao-windows.log` |
| Windows: `npm error EPERM ... esbuild.exe` | Um `npm start` aberto está usando o arquivo. Os scripts atuais encerram o que roda de dentro de `src/frontend/node_modules` antes do `npm ci`; numa versão antiga, feche o `npm start` e rode de novo |
| Windows: `$'\r': command not found` | Os `.sh` foram baixados com quebra de linha do Windows (CRLF). Clone o repositório com o Git (o `.gitattributes` força LF) em vez de baixar o zip, ou rode `git rm --cached -r . && git reset --hard` |
| Windows: o serviço não é registrado/iniciado | Rode o `3-publicar-atualizar.sh` num Git Bash aberto como Administrador |
| Navegador alerta sobre o certificado | Veja [O navegador alerta sobre o certificado](#o-navegador-alerta-sobre-o-certificado), abaixo |
| O site não sobe e a instalação diz que a porta 5443 está em uso | Outro programa está na porta: a instalação diz qual e o PID. Feche esse programa (ou pare o serviço dele) e rode de novo. No Windows pelo PowerShell, a instalação para antes de trocar a versão, sem mudar nada |
| O secret da Pluggy não abre mais (depois de migrar com o `Financas.MigrarSqlite`, trocar de máquina/SO ou apagar `dados\chaves`) | As chaves de proteção de dados são específicas da instalação: informe o Client Secret de novo em Configurações |

### O navegador alerta sobre o certificado

O site usa um certificado HTTPS feito na instalação, só para este computador. A instalação tenta deixá-lo confiável sozinha (`dotnet dev-certs https --trust`); quando não consegue, ela diz isso e mostra os comandos abaixo, já com o caminho do arquivo.

- **Windows:** rode o `3-publicar-atualizar.sh` (ou o `instalar-windows.cmd`) de novo, que recoloca o certificado nos confiáveis.
- **Firefox (em qualquer sistema):** por padrão, ele não usa os certificados do sistema. Em `about:config`, deixe `security.enterprise_roots.enabled` como `true`; ou importe o `.pem` (abaixo) em **Configurações › Privacidade e segurança › Certificados › Ver certificados › Servidores › Importar**. A instalação não muda o Firefox sozinha: ligar essa opção faz ele confiar em todos os certificados do sistema, inclusive os de empresa, e essa escolha é sua.
- **Linux:** a parte pública do certificado (sem a chave) fica em `dados/chaves/financaslocal.pem`. Confie nele uma vez:

  | Onde | Comando |
  |---|---|
  | Debian, Ubuntu e derivados | `sudo cp dados/chaves/financaslocal.pem /usr/local/share/ca-certificates/financaslocal.crt && sudo update-ca-certificates` |
  | Fedora, RHEL, CentOS | `sudo cp dados/chaves/financaslocal.pem /etc/pki/ca-trust/source/anchors/financaslocal.pem && sudo update-ca-trust` |
  | Arch | `sudo trust anchor --store dados/chaves/financaslocal.pem` |
  | Chrome e Chromium (pacote `libnss3-tools` ou `nss-tools`) | `certutil -d sql:$HOME/.pki/nssdb -A -t "P,," -n "Financas Local" -i dados/chaves/financaslocal.pem` |

  O `-t "P,,"` é o jeito de confiar num certificado de servidor autoassinado no Chrome ([Linux Cert Management, Chromium](https://chromium.googlesource.com/chromium/src/+/84.0.4147.135/docs/linux/cert_management.md)).

