# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

**O que fica aqui** (#566): o que vale em qualquer tarefa ou dispara de lado, no meio de outra. O que é de um assunto só está no lugar dele (skill, `CONTRIBUINDO.md` ou `docs/`), com uma linha aqui dizendo quando ler. Regra nova segue o mesmo critério.

## Idioma

**Responda sempre em português do Brasil**, para qualquer pessoa da equipe: respostas, perguntas, resumos de fim de tarefa, descrições de PR e de issue, mensagens de commit e comentários no código. Vale também quando o conteúdo técnico (log, erro, documentação) vier em inglês e depois de a conversa ser resumida. Nomes de comandos, arquivos e APIs ficam como são.

## Simples e enxuto (#769)

Vale em toda tarefa, de qualquer pessoa: é um app local, de um usuário só, e o que não se paga nesse tamanho não entra. Detalhe e exemplos em [docs/SIMPLICIDADE.md](docs/SIMPLICIDADE.md).
- **Código:** abstração, camada, interface, pacote, dependência ou check de CI novo só com um segundo uso real ou uma falha medida. Antes de criar, procure o que já existe e reuse.
- **Texto:** frase curta, cada regra num lugar só (nos outros, um link), sem história nem passo a passo da implementação (isso fica no git e no PR). Limites: ADR até ~1,5 mil palavras; README de pasta de 10 a 20 linhas (#657); entrada do CHANGELOG até ~60 palavras, o que muda para quem usa.
- **Na dúvida, o menor:** prefira a mudança que tira linhas, arquivos ou passos. Tirar um texto de um lugar não é colá-lo inteiro em outro.

## O que é o projeto

Sistema de gerenciamento financeiro pessoal que roda inteiro localmente, em Windows ou Linux (scripts bash no Linux, PowerShell ou `.msi` no Windows, [ADR 0012](docs/adr/0012-instalador-do-windows-em-powershell.md)): back-end em Go (`src/backend`, um executável só, que roda como serviço do sistema; o .NET saiu na #86 e está na tag `v-ultimo-dotnet`), front-end Angular 22 publicado no `wwwroot` ao lado dele (mesma origem, sem CORS, cookies `__Host-`), banco SQLite local (`dados/sql/FinancasLocal.db`) e o gerenciador (`src/gerenciador-go/`, a janela do Windows e do Linux, [ADR 0052](docs/adr/0052-gerenciador-em-go-com-bandeja-e-pagina-local.md)). Dados bancários chegam via Open Finance através do **Meu Pluggy**. A visão funcional está no [README.md](README.md); camadas, pastas e diagramas, no [docs/ARQUITETURA.md](docs/ARQUITETURA.md); o porquê das decisões, nas [ADRs](docs/adr/README.md). Cada pasta principal tem um README curto que aponta para o `docs/ARQUITETURA.md` em vez de repetir: pasta principal nova ganha o dela (#657).

## Comandos

```bash
cd src/backend && go test -race ./...                         # Go (o resto em "Antes de subir", no src/backend/README.md)
cd src/frontend && npm run test:tipos && npm test             # Jest
cd src/frontend && NAVEGADOR=chromium npm run test:afetados   # antes do push: Jest, build e navegador só no que mudou
cd src/frontend && npm start                                  # front-end em dev (http://localhost:4200, proxy /api -> API)
scripts/geral/instalar.sh                                     # instala ou atualiza (etapas 1-3; no Windows, repassa para o .ps1)
```

O que a CI roda, os testes e os limites de tempo: [CONTRIBUINDO.md](CONTRIBUINDO.md#o-que-a-ci-roda). Instalar pelo código e as pastas: [docs/INSTALACAO-PELO-CODIGO.md](docs/INSTALACAO-PELO-CODIGO.md). Os scripts: [scripts/README.md](scripts/README.md).

## Testes

- **Toda regra do Go nasce com teste** ([ADR 0051](docs/adr/0051-testes-do-back-end-em-go-cada-regra-com-teste.md); como, em [CONTRIBUINDO.md](CONTRIBUINDO.md#testes-do-back-end-go)), e bug corrigido, no Go ou no front-end, ganha teste que falha antes da correção.
- **Front-end:** cada comportamento testado num lugar só ([CONTRIBUINDO.md, "Testes rápidos"](CONTRIBUINDO.md#testes-rápidos-padrões-de-ci)). **Mudou tela, CSS, rota ou chamada da API? Use as skills `testar-no-navegador` e `revisar-no-celular`** (#328) antes de dizer que terminou.
- **Mexeu em import, pacote ou pasta nova** (Go ou front-end): rode o teste de arquitetura, que a CI não roda (#476): `go test -tags arquitetura ./internal/arquitetura/` e `npm run test:arquitetura`.
- **Mexeu em teste ou em CI:** tempo antes e depois no PR. Mudança que mexe numa meta dos [requisitos não funcionais](docs/REQUISITOS-NAO-FUNCIONAIS.md) (memória, tamanho, partida) também mede antes e depois.

## Arquitetura (detalhes em docs/ARQUITETURA.md)

- **Front-end** (`src/frontend/src/app`): componente novo com `changeDetection: ChangeDetectionStrategy.Eager`. Desde o Angular 22 o padrão é OnPush, mas o app roda com zone.js e muitas telas guardam estado em campos comuns: sem `Eager`, a tela pode não atualizar, sem erro nenhum. HTTP só pelo `ApiService` (`core/api.service.ts`), nunca `HttpClient` no componente. O resto: skill `angular-no-projeto`.
- **Go** (`src/backend`): o roteiro de todo trabalho é a skill `go-no-projeto` (pacote por assunto, rotas, testes, banco); dinheiro e módulos em "Código Go", abaixo.

## Banco (sem migrations, SQLite)

- **Mudou a estrutura** (tabela, coluna, índice)? Siga [docs/BANCO-DE-DADOS.md, "Como alterar a estrutura"](docs/BANCO-DE-DADOS.md#como-alterar-a-estrutura).
- **Dado pessoal novo:** tabela ou coluna com dado pessoal, ou dado novo que sai do computador, entra em [docs/DADOS-PESSOAIS.md](docs/DADOS-PESSOAIS.md) no mesmo PR.
- **Mudou a cópia, o WAL, a pasta `dados/` ou as chaves?** Atualize o [guia de operação](docs/OPERACAO.md) no mesmo PR, e os passos de restaurar dos workflows "Teste de instalação" (#468) e "Versão (instalador .msi)" (#527), que seguem o guia.
- **Script novo, movido ou mudado:** o índice [scripts/README.md](scripts/README.md) diz quem roda cada um e o que muda junto (Windows, `.msi`); script novo ou movido entra nele.

## Decisões de arquitetura (ADRs) — obrigatório

As decisões ficam em `docs/adr/` (índice, critérios e ciclo de vida em [docs/adr/README.md](docs/adr/README.md); modelo em `docs/adr/0000-modelo.md`). **Antes de implementar** qualquer mudança que adicione ou troque dependência/integração externa (inclusive novo domínio de saída de rede), mude o banco além de uma tabela/coluna rotineira, mexa em segurança, infraestrutura/publicação/CI, ou crie/quebre um padrão de código do projeto:

1. Leia o índice de `docs/adr/README.md` e as ADRs do assunto. Se a mudança contraria uma ADR **Aceita**, avise o usuário antes de seguir — não contorne a decisão em silêncio.
2. **Já existe ADR do assunto:** atualize-a se a mudança só detalha ou ajusta a mesma decisão (consequências, passos práticos). Se a decisão em si mudou, crie uma ADR nova e marque a antiga como `Substituída por NNNN` — nunca apague ADR.
3. **Não existe:** crie `docs/adr/NNNN-titulo-em-kebab-case.md` a partir do modelo, com o próximo número livre, alternativas consideradas e consequências negativas; marque como *inferido* o que não foi confirmado pelo usuário.
4. Atualize o índice em `docs/adr/README.md` e, se mudar o estado atual descrito, `docs/ARQUITETURA.md`.
5. A ADR vai no mesmo commit/PR da mudança. Ao final, diga ao usuário qual ADR foi criada ou atualizada (ou por que a mudança não precisou de uma).

Não precisa de ADR: bugfix, refatoração local, ajuste visual, tela ou endpoint que só segue padrões já registrados.

**Diagramas sempre junto** ([ADR 0055](docs/adr/0055-documentacao-de-arquitetura-em-diagramas-mermaid.md)): mudou um contêiner, uma saída de rede ou uma fronteira? Atualize os diagramas de Contexto, de Contêineres e de implantação em `docs/ARQUITETURA.md` **no mesmo PR**, como o Manual (#177). Pacote novo em `src/backend/internal/` entra na tabela do [diagrama de Componentes](docs/arquitetura/componentes.md) (#464; o teste de arquitetura local confere). Termo novo do domínio entra na lista de [`docs/arquitetura/dominio.md`](docs/arquitetura/dominio.md), e estado novo (enum, `Status`) atualiza o diagrama de estados de lá.

## Convenções de código

- **Manual do site sempre junto (#177):** toda mudança que o usuário vê (tela nova ou alterada, fluxo, cálculo que muda um número, texto de botão) atualiza o Manual (`src/frontend/src/app/paginas/manual/manual-conteudo.ts`) **no mesmo PR**: a seção da tela, os passos e os termos. Se a mudança não aparece para o usuário (refatoração, bug interno), escrever "Manual: não se aplica" na descrição do PR. A CI (job "manual do site") avisa quando falta.
- Nomes em português em todo o código (`XxxService` no Angular, `Servico` e `NovoServico` no Go, `XxxDto`, `SalvarXxxDto`).
- Consultas ao banco **só parametrizadas** (com `?`), nunca SQL montado a partir de texto do usuário.
- **Código Go:** **dinheiro nunca em `float64`**, nem `SUM`/`ORDER BY`/`<`/`>` no SQL em coluna de dinheiro (é `TEXT`): somar e comparar em Go, com decimal. **Funcionalidade nova fora do núcleo financeiro é módulo** ([ADR 0064](docs/adr/0064-modulos-compilados-no-executavel-e-ligados-no-banco.md)): implementa o contrato do `internal/plataforma/modulos`, fica em `internal/modulos/<id>` e segue o checklist do épico #190. O resto: skill `go-no-projeto`.
- Segredos (Client Secret da Pluggy) ficam **criptografados na tabela `Configuracao`** (`internal/plataforma/segredos`: AES-256-GCM, prefixo `v2:`) — nunca em arquivo de config nem devolvidos pela API.
- Formatação: `.editorconfig`.

## UI/UX

**Mexeu em tela, navegação, gráfico, filtro, formulário ou UX? Leia [docs/UI-UX.md](docs/UI-UX.md) antes de propor, revisar ou implementar:** as referências obrigatórias (Shneiderman, Nielsen, WCAG 2.2, NN/g, WAI-ARIA) e os padrões já adotados. Cite o princípio que sustenta cada decisão, separe o que vem das fontes do que é julgamento próprio e diga quando algo não foi validado com uso real.

## Skills e agentes do projeto (`.claude/`)

- **Go:** a skill `go-no-projeto` é o roteiro de todo trabalho em Go; o agente `dev-go` a carrega, para analisar, criar ou revisar assuntos em paralelo (#560).
- **Angular:** a skill `angular-no-projeto` é o roteiro de todo trabalho no front-end; a `testar-no-navegador` confere a tela pronta. O agente `dev-angular` carrega as duas, um por tela (#559).
- **Pedido grande demais para uma vez só:** ofereça a skill `trabalho-longo` em uma frase quando o pedido cita mais de uma issue ou PR, ou um épico; quando é a 2ª issue ou PR na mesma conversa, ou a conversa já foi resumida; ou quando a pessoa fala em "deixar rodando" ou "continuar depois". Não repita se já houver agendamento (`CronList`) ou se a pessoa recusou. O hook `lembrar-trabalho-longo.sh` lembra sozinho em parte desses casos (#280).

## Git / commits

Conventional Commits em português ([tipos e escopos](CONTRIBUINDO.md#mensagens-de-commit)). Trabalho novo parte do `develop`, em `feature/<assunto>` ou `fix/<assunto>`; a `main` é protegida ([branches](CONTRIBUINDO.md#branches)). Mudança só de CI vai num PR `ci:` ou `ci(escopo):`, sem produto junto.

- **Versão: ninguém a sobe no PR.** O número é calculado no build, e o **tipo do título do PR** o decide (`feat` sobe o do meio, o resto o último). Quebra de verdade (o usuário precisa fazer algo à mão, ou não dá para voltar à versão anterior): pergunte ao usuário antes ([CONTRIBUINDO.md, "Versão e CHANGELOG"](CONTRIBUINDO.md#versão-e-changelog)).
- **CHANGELOG:** `feat` e `fix` levam um arquivo `changelog.d/<nome-da-branch>.md` ([formato](changelog.d/LEIA-ME.md)); não mexa no `CHANGELOG.md`.
- **O `.msi` não é gerado no PR** ([ADR 0050](docs/adr/0050-msi-gerado-no-merge-e-nao-no-pr.md)). PR que mexe em `instalador/`, `src/gerenciador-go/`, `src/backend/cmd/financas/` ou `src/backend/internal/plataforma/banco/esquema_banco.go`: rode o workflow "Versão (instalador .msi)" à mão na branch antes do merge e diga no PR que rodou.

**Issues** (comandos em [CONTRIBUINDO.md](CONTRIBUINDO.md#branches)):
- Todo PR para o `develop` leva `Closes #N` na descrição. Branch nova, de preferência criada pela issue: `gh issue develop <N> --base develop --name feature/<assunto> --checkout`.
- **Issue nova já nasce no nome de quem pediu** (`gh issue create --assignee @me`, ou o login de quem pediu). **Exceção, os épicos:** o épico e as histórias dele não são atribuídos a ninguém, nem ao criar, nem depois, nem ao ligar uma história ao épico. Só a história que uma pessoa **pega**, pedindo aqui, vai para o nome dela. Se o usuário pedir para não atribuir uma issue a ele, não atribua.
- **Antes de trabalhar numa issue:** atribuir a quem pediu e pôr a etiqueta "em progresso" (`gh issue edit <N> --add-assignee @me --add-label "em progresso"`). Ao fechar, tirar a etiqueta.
- **Dependência real entre issues vai no GitHub**, como *blocked by*, além do "Depende de: #N" no texto: ao criar uma issue, ao montar as histórias de um épico e ao descobrir uma dependência nova. "De preferência antes" não bloqueia, e ligar ao épico (sub-issue) não substitui o *blocked by*.
- **Não sabe o número da issue:** pergunte ao usuário. Se não houver issue, ofereça criar uma.
- **Backlog e andamento** ficam no Project "ClaudioFinancas" ([users/LucasDesenvGit/projects/2](https://github.com/users/LucasDesenvGit/projects/2)): consulte quando a conversa for sobre backlog, andamento ou o que falta ([campos e comandos](CONTRIBUINDO.md#backlog-no-project)).

## Segurança (específico deste projeto — não genérico)

Tudo roda local (bind em `127.0.0.1:5443`, direto pelo executável do Go — sem IIS/proxy). A única exceção é o **acesso pelo celular** ([ADR 0061](docs/adr/0061-acesso-pelo-celular-na-rede-local.md), `internal/plataforma/celular`): desligado por padrão, ligado com login, abre um segundo servidor só num IPv4 privado da rede de casa, com o mesmo pipeline e a autoridade local. Não abra outra porta nem outro endereço fora dele. Pontos que afetam como escrever código aqui:
- Acesso ao banco é só permissão de arquivo do SO sobre `dados/` — SQLite não tem login/usuário de banco; quem lê/grava é o processo do site, rodando como a conta do serviço do sistema (systemd no Linux, Serviço do Windows no Windows). Sem senha de banco em lugar nenhum.
- Segredos (Client Secret da Pluggy) com a chave em `dados/chaves/segredos.chave`: protegida pelo DPAPI no Windows (`internal/plataforma/segredos/protecao_windows.go`), permissão de arquivo (0600) no Linux.
- CSP restritiva: nada de scripts/recursos de terceiros carregados do front-end.
- **Modelo de ameaças em dia:** uma saída de rede nova, uma fronteira nova (processo, porta, pasta) ou uma mudança no login revisa o [modelo de ameaças](docs/MODELO-DE-AMEACAS.md) no mesmo PR.
- Saídas de rede permitidas e já implementadas: `api.pluggy.ai`, `api.bcb.gov.br`, `olinda.bcb.gov.br` (câmbio PTAX), `query1.finance.yahoo.com`, `api.mercadobitcoin.net` (cripto, ADR 0048), `g1.globo.com`/`feeds.bbci.co.uk`/`www.tabnews.com.br` (só com o módulo Notícias ligado, ADR 0067) (o site) e `api.github.com`/`github.com` (só o gerenciador: versão nova, ADR 0052; e, na instalação de desenvolvimento, o botão "Atualizar para a última do develop", ADR 0040) — qualquer integração externa nova deve seguir o mesmo padrão (HTTPS, dado mínimo enviado, ver a tabela em [docs/SEGURANCA.md](docs/SEGURANCA.md#o-que-sai-para-a-internet) e o diagrama "Como funciona" do [README.md](README.md#como-funciona), que mudam junto).
