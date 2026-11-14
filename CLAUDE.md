# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

**O que fica aqui** (#566): o que vale em qualquer tarefa ou dispara de lado, no meio de outra. O que é de um assunto só está no lugar dele (skill, `CONTRIBUINDO.md` ou `docs/`), com uma linha aqui dizendo quando ler. Regra nova segue o mesmo critério.

## Idioma

**Responda sempre em português do Brasil**, para qualquer pessoa da equipe: respostas, perguntas, resumos de fim de tarefa, descrições de PR e de issue, mensagens de commit e comentários no código. Vale também quando o conteúdo técnico (log, erro, documentação) vier em inglês e depois de a conversa ser resumida. Nomes de comandos, arquivos e APIs ficam como são.

## O que é o projeto

Sistema de gerenciamento financeiro pessoal que roda inteiro localmente, em Windows ou Linux (scripts bash no Linux, PowerShell ou `.msi` no Windows, [ADR 0012](docs/adr/0012-instalador-do-windows-em-powershell.md)): back-end em Go (`src/backend`, um executável só, que roda como serviço do sistema: Serviço do Windows ou systemd; o .NET saiu na #86 e está na tag `v-ultimo-dotnet`), front-end Angular 22, banco SQLite local (`dados/sql/FinancasLocal.db`). Dados bancários chegam via Open Finance através do **Meu Pluggy**. Veja [README.md](README.md) para a visão funcional completa (painel, transações, orçamentos, metas, investimentos, relatórios) e [docs/ARQUITETURA.md](docs/ARQUITETURA.md) para o diagrama de camadas. O porquê das decisões está nas ADRs em [docs/adr/](docs/adr/README.md). **O mapa das pastas para pessoas** é o README de cada uma (`src/backend`, `src/frontend`, `src/gerenciador-go`, `banco`, `instalador`, `docs`): pasta principal nova ganha o dela, de 10 a 20 linhas, apontando para o `docs/ARQUITETURA.md` em vez de repetir (#657).

## Comandos

```bash
cd src/backend && go test -race ./...                      # back-end Go ("Antes de subir" do src/backend/README.md)
cd src/frontend && npm run build                              # front-end (produção)
cd src/frontend && npm run test:tipos && npm test             # Jest
cd src/frontend && NAVEGADOR=chromium npm run test:afetados   # antes do push: Jest + navegador só no que mudou (#286)
cd src/frontend && npm start                                  # front-end em dev (http://localhost:4200, proxy /api -> API)
scripts/geral/2-criar-banco.sh                                # banco (sem migrations)
scripts/geral/3-publicar-atualizar.sh                         # compila, publica, certificado e serviço do sistema
scripts/geral/instalar.sh                                     # etapas 1-3 (no Windows, repassa para scripts/windows/instalar-windows.ps1; sem bash: instalar-windows.cmd, ADR 0012)
```

O que a CI roda, os testes rápidos e os limites de tempo: [CONTRIBUINDO.md](CONTRIBUINDO.md#o-que-a-ci-roda). Desenvolvimento, migrar do SQL Server e pastas: [docs/INSTALACAO-PELO-CODIGO.md](docs/INSTALACAO-PELO-CODIGO.md).

**O `.msi` não é gerado no PR, só no merge para o develop** ([ADR 0050](docs/adr/0050-msi-gerado-no-merge-e-nao-no-pr.md)). PR que mexe em `instalador/`, `src/gerenciador-go/`, `src/backend/cmd/financas/` ou `src/backend/internal/plataforma/banco/esquema_banco.go`: rode o workflow "Versão (instalador .msi)" à mão na branch antes do merge e diga no PR que rodou. Mudança só de CI vai num PR `ci:` ou `ci(escopo):` (ADR 0039), sem produto junto.

## Testes

- **Front-end:** cada comportamento testado num lugar só, lógica no Jest e desenho, CSS, rota e CSP no navegador; regras, tempos e pegadinhas em [CONTRIBUINDO.md, "Testes rápidos"](CONTRIBUINDO.md#testes-rápidos-padrões-de-ci). **Mudou tela, CSS, rota ou chamada da API? Use a skill `testar-no-navegador`** antes de dizer que terminou, e a `revisar-no-celular` (largura, toque, alvo de 24 px, teclado virtual; #328) em toda mudança de tela. Mexeu em teste ou em CI: tempo antes e depois no PR.
- **Go: toda regra nasce com teste** ([ADR 0051](docs/adr/0051-testes-do-back-end-em-go-cada-regra-com-teste.md)): unitário em tabela, SQLite real, `httptest`, só a biblioteca padrão; bug corrigido ganha teste que falha antes. Detalhes em [CONTRIBUINDO.md, "Testes do back-end (Go)"](CONTRIBUINDO.md#testes-do-back-end-go).
- **Mexeu em import, pacote ou pasta nova** (Go ou front-end): rode o teste de arquitetura, `go test -tags arquitetura ./internal/arquitetura/` e `npm run test:arquitetura` (#476). Ele é só local, por decisão de 29/10/2026, e a CI não o roda. Dependência nova de propósito entra na tabela do teste, com o motivo, no mesmo PR.

## Arquitetura (detalhes em docs/ARQUITETURA.md)

Angular é publicado no `wwwroot`, ao lado do executável do Go (`financas-go`): mesma origem, sem CORS, cookies `__Host-`. Um pacote por assunto em `src/backend/internal/<area>/<assunto>` (áreas iguais às do front-end, mais `openfinance` e `plataforma`), rotas juntadas em `internal/api` ([ADR 0056](docs/adr/0056-arquitetura-do-codigo-go-camadas-por-assunto-e-ddd-estrategico.md)); pastas em [docs/ARQUITETURA.md](docs/ARQUITETURA.md).

- **Front-end** (`src/frontend/src/app`): componente novo com `changeDetection: ChangeDetectionStrategy.Eager`. Desde o Angular 22 o padrão é OnPush, mas o app roda com zone.js e muitas telas guardam estado em campos comuns: sem `Eager`, a tela pode não atualizar, sem erro nenhum. HTTP só pelo `ApiService` (`core/api.service.ts`), nunca `HttpClient` no componente. Página grande ([ADR 0059](docs/adr/0059-paginas-grandes-divididas-em-tela-apresentacao-e-funcoes-puras.md)): a tela guarda o estado e fala com a API, os componentes de apresentação só recebem `input()` e avisam por `output()`, e a regra vai para funções puras em `<tela>.ts`, testadas sem `TestBed`.
- **Gerenciador** (`src/gerenciador-go/`, a janela do Windows e do Linux em Go com o Wails, [ADR 0052](docs/adr/0052-gerenciador-em-go-com-bandeja-e-pagina-local.md)): o de C# saiu na #86.

## Banco (sem migrations, SQLite)

- `banco/01-estrutura.sql` é idempotente (`CREATE TABLE/INDEX IF NOT EXISTS`, tabela nova no fim, conferido pelo `scripts/ci/banco/verificar-sql.py`); `02-dados-iniciais.sql` traz o padrão. O site aplica os dois ao iniciar (`-aplicar-esquema`, `internal/plataforma/banco/esquema_banco.go`, com uma cópia embutida dos scripts que o `go generate` mantém em dia). Nunca apagar/renomear coluna em uso sem bloco de transição.
- **Coluna nova numa tabela existente:** `adicionar_coluna_se_ausente` em `scripts/geral/2-criar-banco.sh`, não no SQL. Uma chamada por linha, argumentos entre aspas duplas, porque o `instalar-windows.ps1` e o Go (`internal/plataforma/banco/esquema`) leem essas linhas.
- **Mudou a estrutura:** o SQL, o `2-criar-banco.sh`, `go generate ./...` em `src/backend` (a cópia embutida e as consultas do `sqlc`), `python3 scripts/dev/gerar-diagrama-banco.py` (tabela nova entra em `ASSUNTOS`) e `feat(banco)!:` no commit. Passos em [docs/BANCO-DE-DADOS.md](docs/BANCO-DE-DADOS.md).
- **Dado pessoal novo:** tabela ou coluna com dado pessoal, ou dado novo que sai do computador, entra em [docs/DADOS-PESSOAIS.md](docs/DADOS-PESSOAIS.md) no mesmo PR.
- **Mudou a cópia, o WAL, a pasta `dados/` ou as chaves?** Atualize o [guia de operação](docs/OPERACAO.md) no mesmo PR, e os passos de restaurar dos workflows "Teste de instalação" (#468) e "Versão (instalador .msi)" (#527), que seguem o guia.
- **Scripts (#154):** `scripts/geral/` é o bash do Linux e do macOS; no Windows, o `instalar.sh`, o `3-publicar-atualizar.sh` e o `parar.sh` só repassam para o `.ps1` de `scripts/windows/`, e só o `2-criar-banco.sh` e o `4-backup.sh` rodam em bash (Git Bash); `scripts/windows/` e `scripts/linux/`, o que só roda num deles; `scripts/dev/`, as ferramentas de quem desenvolve; `scripts/ci/` (em `versao/`, `go/`, `front/` e `banco/`), o que a CI roda; os `.cmd` na raiz. O índice de todos, com quem roda cada um, é o [scripts/README.md](scripts/README.md): script novo ou movido entra nele. As etapas 1 e 3 do Windows existem só no `instalar-windows.ps1` (e as pastas e mensagens comuns, no `scripts/windows/comum.ps1`); mudou o `2-criar-banco.sh`? O `.ps1` lê dele a lista de colunas novas, e o `4-backup.sh` precisa continuar rodando no Git Bash. Serviço, certificado ou pastas de dados também existem no `instalador/configurar.ps1` (.msi, ADR 0029): confira se mudam juntos.

## Decisões de arquitetura (ADRs) — obrigatório

As decisões ficam em `docs/adr/` (índice, critérios e ciclo de vida em [docs/adr/README.md](docs/adr/README.md); modelo em `docs/adr/0000-modelo.md`). **Antes de implementar** qualquer mudança que adicione ou troque dependência/integração externa (inclusive novo domínio de saída de rede), mude o banco além de uma tabela/coluna rotineira, mexa em segurança, infraestrutura/publicação/CI, ou crie/quebre um padrão de código do projeto:

1. Leia o índice de `docs/adr/README.md` e as ADRs do assunto. Se a mudança contraria uma ADR **Aceita**, avise o usuário antes de seguir — não contorne a decisão em silêncio.
2. **Já existe ADR do assunto:** atualize-a se a mudança só detalha ou ajusta a mesma decisão (consequências, passos práticos). Se a decisão em si mudou, crie uma ADR nova e marque a antiga como `Substituída por NNNN` — nunca apague ADR.
3. **Não existe:** crie `docs/adr/NNNN-titulo-em-kebab-case.md` a partir do modelo, com o próximo número livre, alternativas consideradas e consequências negativas; marque como *inferido* o que não foi confirmado pelo usuário.
4. Atualize o índice em `docs/adr/README.md` e, se mudar o estado atual descrito, `docs/ARQUITETURA.md`.
5. A ADR vai no mesmo commit/PR da mudança. Ao final, diga ao usuário qual ADR foi criada ou atualizada (ou por que a mudança não precisou de uma).

Não precisa de ADR: bugfix, refatoração local, ajuste visual, tela ou endpoint que só segue padrões já registrados.

**Requisitos não funcionais** ([docs/REQUISITOS-NAO-FUNCIONAIS.md](docs/REQUISITOS-NAO-FUNCIONAIS.md)): mudança que mexe numa meta (memória, tamanho, partida) mede antes e depois no PR, como já se faz com o tempo dos testes. Partida e telas: `python3 scripts/dev/medir-desempenho.py --api <pasta publicada>`.

**Diagramas sempre junto** ([ADR 0055](docs/adr/0055-documentacao-de-arquitetura-em-diagramas-mermaid.md)): mudou um contêiner, uma saída de rede ou uma fronteira? Atualize os diagramas de Contexto, de Contêineres e de implantação em `docs/ARQUITETURA.md` **no mesmo PR**, como o Manual (#177). Pacote novo em `src/backend/internal/` entra na tabela do [diagrama de Componentes](docs/arquitetura/componentes.md) (#464; o teste de arquitetura local confere). Termo novo do domínio entra na lista de [`docs/arquitetura/dominio.md`](docs/arquitetura/dominio.md), e estado novo (enum, `Status`) atualiza o diagrama de estados de lá.

## Convenções de código

- **Manual do site sempre junto (#177):** toda mudança que o usuário vê (tela nova ou alterada, fluxo, cálculo que muda um número, texto de botão) atualiza o Manual (`src/frontend/src/app/paginas/manual/manual-conteudo.ts`) **no mesmo PR**: a seção da tela, os passos e os termos. Se a mudança não aparece para o usuário (refatoração, bug interno), escrever "Manual: não se aplica" na descrição do PR. A CI (job "manual do site") avisa quando falta. O teste `manual.spec.ts` já exige uma seção por tela.
- Nomes em português em todo o código (`XxxService` no Angular, `Servico` e `NovoServico` no Go, `XxxDto`, `SalvarXxxDto`).
- Consultas ao banco **só parametrizadas** (`?` ou o código gerado pelo `sqlc`), nunca SQL montado a partir de texto do usuário.
- **Código Go:** assunto novo ganha pacote em `src/backend/internal/<area>/<assunto>` e uma linha em `internal/api`, com casos de contrato (`testes/contrato`). **Funcionalidade nova fora do núcleo financeiro é módulo** ([ADR 0064](docs/adr/0064-modulos-compilados-no-executavel-e-ligados-no-banco.md)): implementa o contrato do `internal/plataforma/modulos`, fica em `internal/modulos/<id>` e segue o checklist do épico #190. **Dinheiro nunca em `float64`**, nem `SUM`/`ORDER BY`/`<`/`>` no SQL em coluna de dinheiro (é `TEXT`): somar e comparar em Go, com decimal. O resto (bibliotecas, consultas, formato do banco, estilo, camadas) está na skill `go-no-projeto` e nas ADRs [0049](docs/adr/0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md) e [0056](docs/adr/0056-arquitetura-do-codigo-go-camadas-por-assunto-e-ddd-estrategico.md).
- Segredos (Client Secret da Pluggy, token da brapi) ficam **criptografados na tabela `Configuracao`** (`internal/plataforma/segredos`: AES-256-GCM, prefixo `v2:`) — nunca em arquivo de config nem devolvidos pela API.
- Formatação: `.editorconfig`.

## UI/UX

**Mexeu em tela, navegação, gráfico, filtro, formulário ou UX? Leia [docs/UI-UX.md](docs/UI-UX.md) antes de propor, revisar ou implementar:** as referências obrigatórias (Shneiderman, Nielsen, WCAG 2.2, NN/g, WAI-ARIA) e os padrões já adotados. Cite o princípio que sustenta cada decisão, separe o que vem das fontes do que é julgamento próprio e diga quando algo não foi validado com uso real.

## Skills e agentes do projeto (`.claude/`)

- **Go:** a skill `go-no-projeto` é o roteiro de todo trabalho em Go (funcionalidade nova, mudança, revisão). A `migrar-para-go` era a da migração do épico #70, que terminou na #86: serve só para consultar como um assunto foi portado do C# (que está na tag `v-ultimo-dotnet`). O agente `dev-go` carrega a `go-no-projeto`; serve para analisar, criar ou revisar assuntos em paralelo (#560).
- **Angular:** a skill `angular-no-projeto` é o roteiro de todo trabalho no front-end (tela nova, mudança, avaliação de UI/UX, revisão); a `testar-no-navegador` confere a tela pronta. O agente `dev-angular` carrega as duas, para avaliar telas em paralelo (um agente por tela), implementar ou revisar (#559).
- **Pedido grande demais para uma vez só:** ofereça a skill `trabalho-longo` em uma frase quando o pedido cita mais de uma issue ou PR, ou um épico; quando é a 2ª issue ou PR na mesma conversa, ou a conversa já foi resumida; ou quando a pessoa fala em "deixar rodando" ou "continuar depois". Não repita se já houver agendamento (`CronList`) ou se a pessoa recusou. O hook `lembrar-trabalho-longo.sh` lembra sozinho em parte desses casos (#280).

## Git / commits

Conventional Commits em português (fluxo de branches, comandos e Project em [CONTRIBUINDO.md](CONTRIBUINDO.md)). Escopos usados: `api`, `web`, `banco`, `scripts`, `open-finance`, `investimentos`, `relatorios`. Branch `main` é protegida (só recebe merge de `develop`/`hotfix/*`); trabalho novo parte de `develop` em `feature/<assunto>` ou `fix/<assunto>`.

**Versão: só no `Directory.Build.props`** (API e gerenciador); a tag `vX.Y.Z` tem de ser igual. **Todo PR para o `develop` sobe esse número, pelo tipo do título** ([ADR 0039](docs/adr/0039-versao-sobe-a-cada-pr-pelo-tipo-do-commit.md); o check "versão subiu" da CI confere):
- `feat` (inclusive `feat(banco)!` que só acrescenta tabela ou coluna) → o número do meio (2.2.0 → 2.3.0);
- `fix`, `docs`, `build`, `ci`, `refactor`, `perf`, `test`, `chore`, `style` → o último (2.2.0 → 2.2.1);
- quebra de verdade (o usuário precisa fazer algo à mão, ou não dá para voltar à versão anterior) → o primeiro (3.0.0); pergunte ao usuário antes.
- Outro PR subiu antes? `scripts/dev/atualizar-pr.sh <PR>` traz o `develop` e resolve a versão e o CHANGELOG (#587); conflito em outro arquivo, `git merge origin/develop` à mão. Diga ao usuário, no fim, de qual para qual versão foi.

**`CHANGELOG.md`: uma seção por versão** (#294). A entrada de um PR vai na seção da versão dele: `## [X.Y.Z] — AAAA-MM-DD · desenvolvimento`, criada no topo se não existir, **dentro** do `### Funcionalidades`/`### Correções`/`### Publicação e documentação` que já existe nela (nunca outro título de mesmo tipo). Ao trazer o `develop` e subir a versão de novo, **mova a entrada para a seção da versão nova**. Antes do push, rode `python3 scripts/ci/versao/verificar-changelog.py` (`--corrigir` junta tipos repetidos). As notas das versões e a janela "O que mudou" do gerenciador saem dele: seção errada vira nota errada para quem atualiza.

**Issues** (o branch padrão é o `develop`, #142):
- Todo PR para o `develop` leva `Closes #N` na descrição: o GitHub vincula o PR à issue e a fecha no merge.
- **Branch nova:** de preferência criada pela issue, com `gh issue develop <N> --base develop --name feature/<assunto> --checkout`.
- **Issue nova já nasce no nome de quem pediu** (`gh issue create --assignee @me`, ou o login de quem pediu). **Exceção, os épicos:** o épico e as histórias dele não são atribuídos a ninguém, nem ao criar, nem depois, nem ao ligar uma história ao épico. Só a história que uma pessoa **pega**, pedindo aqui, vai para o nome dela, com a etiqueta "em progresso". Se o usuário pedir para não atribuir uma issue a ele, não atribua.
- **Antes de trabalhar numa issue:** atribuir a quem pediu e pôr a etiqueta "em progresso" (`gh issue edit <N> --add-assignee @me --add-label "em progresso"`). Ao fechar, tirar a etiqueta.
- **Dependência entre issues vai no GitHub, não só no texto:** issue que só pode começar depois de outra pronta ganha *blocked by* (comando em [CONTRIBUINDO.md](CONTRIBUINDO.md#branches)), além do "Depende de: #N" no texto. Vale ao criar uma issue, ao montar as histórias de um épico e ao descobrir uma dependência nova. Só dependência real: "de preferência antes" não bloqueia, e ligar ao épico (sub-issue) não substitui o *blocked by*.
- **Não sabe o número da issue:** pergunte ao usuário. Se não houver issue, ofereça criar uma.
- **Backlog e andamento** ficam no Project "ClaudioFinancas" ([users/LucasDesenvGit/projects/2](https://github.com/users/LucasDesenvGit/projects/2)): consulte quando a conversa for sobre backlog, andamento ou o que falta (campos e comandos em [CONTRIBUINDO.md](CONTRIBUINDO.md#backlog-no-project)).

## Segurança (específico deste projeto — não genérico)

Tudo roda local (bind em `127.0.0.1:5443`, direto pelo executável do Go — sem IIS/proxy). A única exceção é o **acesso pelo celular** ([ADR 0061](docs/adr/0061-acesso-pelo-celular-na-rede-local.md), `internal/plataforma/celular`): desligado por padrão, ligado com login, abre um segundo servidor só num IPv4 privado da rede de casa, com o mesmo pipeline e a autoridade local. Não abra outra porta nem outro endereço fora dele. Pontos que afetam como escrever código aqui:
- Acesso ao banco é só permissão de arquivo do SO sobre `dados/` — SQLite não tem login/usuário de banco; quem lê/grava é o processo do site, rodando como a conta do serviço do sistema (systemd no Linux, Serviço do Windows no Windows). Sem senha de banco em lugar nenhum.
- Segredos (Client Secret da Pluggy, token da brapi) com a chave em `dados/chaves/segredos.chave`: protegida pelo DPAPI no Windows (`internal/plataforma/segredos/protecao_windows.go`), permissão de arquivo (0600) no Linux.
- CSP restritiva: nada de scripts/recursos de terceiros carregados do front-end.
- **Modelo de ameaças em dia:** uma saída de rede nova, uma fronteira nova (processo, porta, pasta) ou uma mudança no login revisa o [modelo de ameaças](docs/MODELO-DE-AMEACAS.md) no mesmo PR.
- Saídas de rede permitidas e já implementadas: `api.pluggy.ai`, `api.bcb.gov.br`, `olinda.bcb.gov.br` (câmbio PTAX), `query1.finance.yahoo.com`/`brapi.dev`, `api.mercadobitcoin.net` (cripto, ADR 0048), `g1.globo.com`/`feeds.bbci.co.uk`/`www.tabnews.com.br` (só com o módulo Notícias ligado, ADR 0067) (o site) e `api.github.com`/`github.com` (só o gerenciador: versão nova, ADR 0052; e, na instalação de desenvolvimento, o botão "Atualizar para a última do develop", ADR 0040) — qualquer integração externa nova deve seguir o mesmo padrão (HTTPS, dado mínimo enviado, ver a tabela em [docs/SEGURANCA.md](docs/SEGURANCA.md#o-que-sai-para-a-internet) e o diagrama "Como funciona" do [README.md](README.md#como-funciona), que mudam junto).
