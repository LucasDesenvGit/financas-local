# Testes de contrato

A rede de segurança da migração para Go ([#72](https://github.com/LucasDesenvGit/financas-local/issues/72), [ADR 0049](../../../../docs/adr/0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md)). O que o back-end .NET responde hoje fica gravado em `esperado/` e vira a régua do Go: **mesmo status, mesmo JSON e o banco no mesmo estado**, caso a caso.

São *characterization tests*: não dizem o que o sistema deveria fazer, congelam o que ele faz. Se o .NET tem um comportamento estranho, o esperado também tem, de propósito. Diferença de comportamento vira issue, não correção no contrato.

| Pasta ou arquivo | O que tem |
|---|---|
| `casos/NN-assunto.json` | o roteiro: requisições em ordem, um arquivo por assunto |
| `esperado/NN-assunto.json` | o que o .NET respondeu e gravou em cada caso (*golden files*), já normalizado |
| `dados/base.sql` | o banco fictício: dados **inventados**, nunca de uma pessoa de verdade |
| `rodar-dotnet.sh` | monta o banco, sobe o .NET (com o Go atrás dele) sem rede e com o relógio controlado, e roda o comparador |
| `assuntos-no-go.txt` | os assuntos com código Go que a rodada contra o Go confere (`--go`, e a CI), um por linha |
| `*.go` | o gravador e o comparador (Go puro; `go test ./...` roda os testes deles sem servidor) |

Hoje: `00-auth` (40 casos), `01-parametros` (14), `02-cambio` (15), `03-cripto` (2), `04-contas` (63), `05-categorias` (33), `06-regras` (38), `07-transacoes` (205), `11-investimentos` (105), `17-erros` (52) e `90-go` (4, rotas que só existem no Go). **Cada etapa do épico grava os casos do próprio assunto, no próprio PR, antes do código Go** (ADR 0049), no número reservado para ele ([tabela](#numeração-dos-arquivos-de-casos)).

## Rodar contra o .NET

Precisa de Linux, do SDK do .NET 10, do Go, do `sqlite3`, do `unshare` (util-linux) e da libfaketime.

```bash
dotnet build src/backend/Financas.Api/Financas.Api.csproj -c Release
src/backend-go/testes/contrato/rodar-dotnet.sh               # compara com esperado/
src/backend-go/testes/contrato/rodar-dotnet.sh --atualizar   # grava esperado/
```

- **libfaketime:** `apt-get install faketime` (Debian e Ubuntu). Sem o pacote, compile a [fonte](https://github.com/wolfcw/libfaketime) (`make -C src`) e aponte `FAKETIME_LIB=/caminho/libfaketime.so.1`.
- **Ubuntu 24.04:** o `unshare -rn` precisa de `sudo sysctl -w kernel.apparmor_restrict_unprivileged_userns=0` (a CI faz isso).
- `DOTNET=~/.dotnet/dotnet` se o `dotnet` não estiver no PATH. `MANTER_PASTA=1` guarda a pasta temporária, com o banco e o log da API (`api.log`).
- A API sobe na porta `5451` (`PORTA=...` para trocar). A execução inteira leva uns 6 s fora o build.
- O roteiro compila o Go (`cmd/financas`, sem cgo e com a versão do `Directory.Build.props`, como na publicação) e o põe onde o .NET procura (`Go:Executavel`). O .NET o sobe como processo filho (#231) e só começa a comparar depois de o Go avisar a porta.

Cada execução começa do zero: banco novo, processo novo. O roteiro depende da ordem (o `00-auth` cria o usuário e deixa a sessão `principal` logada para os seguintes), então ele sempre roda inteiro.

## Rodar contra o Go atrás do .NET

É o jeito de cada assunto conferir o código Go dele (#363): o mesmo roteiro, os **mesmos esperados**, e o .NET repassando ao Go os prefixos de `PREFIXOS_GO`, separados por `;`:

```bash
PREFIXOS_GO="/api/cambio;/api/cripto" src/backend-go/testes/contrato/rodar-dotnet.sh
src/backend-go/testes/contrato/rodar-dotnet.sh --go   # os assuntos ligados em assuntos-no-go.txt (o que a CI roda)
```

- O resto continua no .NET, e as rotas que só existem no Go (`/api/go`) vão sempre para o Go. O roteiro mostra o que o Go atendeu: `Go atrás do .NET, atendendo: /api/go;/api/cambio;/api/cripto`.
- **O relógio do Go é o mesmo do .NET.** A libfaketime não age sobre o Go, que lê o relógio direto do kernel (vDSO), sem passar pela libc. Por isso o roteiro passa ao .NET `Go:InicioDoRelogio=2026-10-15T01:00:00Z`, e o .NET sobe o Go com `-inicio-do-relogio`: esse instante, mais o tempo desde a partida do .NET. O Go começa ali e anda dali, como o `FAKETIME="@..."` (`relogio.APartirDe`). Se o Go reiniciar no meio, o relógio dele continua, sem voltar ao início.
- **Nunca grave os esperados a partir do Go.** O `--atualizar` com `PREFIXOS_GO` ou `--go` é recusado: o .NET é a referência. A exceção são as rotas que só existem no Go (abaixo).
- Diferença que só aparece com o Go é bug do Go (ou diferença de comportamento a discutir numa issue), nunca motivo para mexer no esperado.

### Como cada assunto liga o contrato contra o Go

1. **Casos primeiro** (passo 1 da ADR 0049): o arquivo do assunto, no número reservado dele (tabela abaixo), gravado com `--atualizar` contra o .NET, sem `PREFIXOS_GO`.
2. **Código Go:** o pacote `internal/<assunto>` exporta `Registrar(mux, ...)` e ganha **uma linha** no `internal/api/api.go`.
3. **Contrato contra o Go:** `PREFIXOS_GO="<prefixos do assunto>" rodar-dotnet.sh` tem de passar com os mesmos esperados, duas vezes.
4. **Na CI:** o PR tira o `#` da linha do assunto em `assuntos-no-go.txt`. O job "contrato (.NET)" roda o contrato duas vezes: com o .NET respondendo (a referência) e com `--go`, que passa pelo Go todos os assuntos ligados ali. Sem mexer no `ci.yml`, que faria a CI rodar todos os testes do front-end. As linhas já estão reservadas, com uma linha em branco entre elas: dois PRs em paralelo mexem cada um na sua linha, sem conflito no merge.
5. **Ligar no site** (passo 2, PR à parte): os prefixos na entrada do assunto em `Go:Prefixos`, no `appsettings.json` (uma entrada por assunto, já reservada, vazia). O contrato não depende disso: ele escolhe os prefixos pelo `PREFIXOS_GO` (ou `--go`) e ignora o `appsettings.json`.
6. **C# apagado** (passo 3): sem o .NET para responder, os prefixos do assunto saem do `assuntos-no-go.txt` e vão para `prefixos_so_no_go` no `rodar-dotnet.sh`, que vão sempre para o Go.

### Rotas que só existem no Go

O diagnóstico `/api/go/saude` (#231) e a funcionalidade nova feita só em Go (`CLAUDE.md`) não têm .NET a imitar. Os prefixos delas ficam em `prefixos_so_no_go` no `rodar-dotnet.sh`, vão sempre para o Go, e o esperado é o que o Go respondeu: o caso confere que nada muda sem querer. O `90-go` confere também que o relógio de teste chega até a rota: com o relógio do computador, `agora` cai fora da janela e `hoje` não é 14/10.

### O Go sozinho, sem o .NET na frente

O login (#75) não passa pelo repasse: `/api/auth` fica no .NET até a inversão (#85). O Go sozinho tem o login dele (`internal/autenticacao`), e o roteiro o compara com os mesmos esperados do .NET:

```bash
src/backend-go/testes/contrato/rodar-dotnet.sh --go-sozinho                       # os arquivos de CONTRATO_SOMENTE (00-auth,01-parametros,02-cambio,03-cripto,17-erros)
CONTRATO_SOMENTE="00-auth" src/backend-go/testes/contrato/rodar-dotnet.sh --go-sozinho
```

- Monta o mesmo banco fictício, compila o Go com a versão do `Directory.Build.props`, sobe-o sozinho com o relógio de teste (`-inicio-do-relogio`) e um certificado em memória, e roda o comparador só nos arquivos de `CONTRATO_SOMENTE` (opção `-somente` do `go test`).
- Não precisa do .NET, da libfaketime nem do `unshare`: o Go tem o próprio relógio de teste, e o login não sai para a rede. Roda também no Git Bash do Windows, com o `sqlite3` no PATH.
- **Só os assuntos que não dependem das rotinas de partida do `Program.cs`**: sem o .NET, elas não rodam. O login, os parâmetros e os erros não dependem de nenhuma (o Go sozinho tem o próprio gravador de erros, que limpa os antigos já na partida, como o .NET). Assunto novo entra em `CONTRATO_SOMENTE` quando o Go o atende sozinho, **na mesma ordem da rodada completa**: os casos de um arquivo deixam rastro no banco que os seguintes veem (o câmbio, sem internet, deixa 4 avisos abertos que o `17-erros` conta; #416).
- **Sem rede, sem `unshare`:** o roteiro sobe o Go com `HTTPS_PROXY=http://127.0.0.1:9`, uma porta fechada. O `http.Client` do Go segue o proxy ([`ProxyFromEnvironment`](https://pkg.go.dev/net/http#ProxyFromEnvironment)), e o endereço de loopback, que é o do teste, nunca passa por ele: toda chamada para a internet falha como no .NET dentro do `unshare -rn`.
- Sem o roteiro, à mão: suba o Go (`go run ./cmd/financas -endereco 127.0.0.1:5453 -certificado-temporario -wwwroot internal/servidor/testdata/wwwroot -banco <db> -inicio-do-relogio 2026-10-15T01:00:00Z`) e rode `go test ./testes/contrato -count=1 -args -alvo=https://127.0.0.1:5453 -banco=<db> -inicio=2026-10-15T01:00:00Z -somente=00-auth,01-parametros,02-cambio,03-cripto,17-erros`, com `HTTPS_PROXY=http://127.0.0.1:9` no Go. O `-inicio` do comparador e o `-inicio-do-relogio` do Go são o mesmo instante.

## Adicionar um caso

1. Escreva o caso no `casos/NN-assunto.json` do assunto (ou crie o arquivo, com o número da [tabela](#numeração-dos-arquivos-de-casos); o número dá a ordem).
2. Grave os esperados: `rodar-dotnet.sh --atualizar`.
3. Leia a diferença de `esperado/` no `git diff`: é a descrição exata do que o .NET faz. Confira que nenhum esperado de outro assunto mudou sem motivo.
4. Rode de novo **sem** `--atualizar`, duas vezes: tem de passar as duas.

Cubra os erros também (400, 401, 404, 409, 429), não só o caminho feliz. Teste novo que passa uma vez e falha na outra é instável: descubra o que muda e normalize (ou mude o caso), nunca afrouxe a comparação.

### Numeração dos arquivos de casos

Cada assunto do épico #70 já tem o número dele, para os assuntos feitos em paralelo (#75, #77, #81) não disputarem o mesmo arquivo nem renumerarem os dos outros. O número segue a ordem de trabalho do épico, que é também a ordem dos dados: o painel (#80) lê o que as transações (#78) gravam, e não o contrário. Use o seu número mesmo que os anteriores ainda não existam.

| Arquivo | Assunto | Issue | Prefixos | Entrada em `Go:Prefixos` |
|---|---|---|---|---|
| `00-auth` | login, sessão, CSRF, limites | #75 | `/api/auth` | nenhuma: fica no .NET até a inversão (#85) |
| `01-parametros` | parâmetros e configurações | #76 | `/api/parametros` | nenhuma: entra na inversão (#85) |
| `02-cambio` | câmbio (PTAX) | #81 | `/api/cambio` | `cambio` |
| `03-cripto` | cripto | #81 | `/api/cripto` | `cambio` |
| `04-contas` | contas e conferência do banco | #77 (contas), #445 (conferência) | `/api/contas`, `/api/conferencia-banco` | `cadastros` |
| `05-categorias` | categorias | #77 | `/api/categorias` | `cadastros` |
| `06-regras` | regras e regras de descrição | #77 | `/api/regras`, `/api/regras-descricao` | `cadastros` |
| `07-transacoes` | transações | #78 | `/api/transacoes` | `transacoes` |
| `08-orcamentos` | orçamentos | #79 | `/api/orcamentos` | `planejamento` |
| `09-metas` | metas | #79 | `/api/metas` | `planejamento` |
| `10-contas-agendadas` | contas a pagar | #79 | `/api/contas-agendadas` | `planejamento` |
| `11-investimentos` | carteira, reservas, investimentos | #82 | `/api/carteira`, `/api/reservas`, `/api/investimentos` | `investimentos` |
| `12-conferencia-investimentos` | conferir investimentos | #299 | `/api/carteira/conferencia` (dentro de `/api/carteira`: no site, a entrada `investimentos` só liga com a #299 pronta; no contrato, a #82 passa os caminhos dela, mais compridos) | `investimentos` |
| `13-painel` | painel | #80 | `/api/painel` | `painel` |
| `14-relatorios` | relatórios | #80 | `/api/relatorios` | `painel` |
| `15-emprestimos` | empréstimos | #80 | `/api/emprestimos` | `painel` |
| `16-open-finance` | Open Finance (Pluggy) | #83 | `/api/open-finance` | `open-finance` |
| `17-erros` | erros do sistema | #298 | `/api/erros` | nenhuma: fica no .NET até a inversão (#85); o Go sozinho atende (`--go-sozinho`) |
| `20` a `89` | funcionalidade nova feita só em Go (`CLAUDE.md`), por ordem de chegada | — | os dela | uma nova, logo depois da `go` |
| `90-go` | diagnóstico da convivência (`/api/go/saude`) | #231, #363 | `/api/go` | `go` |

- **Assuntos feitos ao mesmo tempo não dependem uns dos outros.** Um arquivo pode usar o `base.sql`, a sessão `principal` do `00-auth` e o que um arquivo de número menor **já mesclado** deixou no banco; nunca o de um assunto ainda em andamento.
- **Depois de trazer o `develop`**, rode o contrato inteiro de novo. Se um esperado seu mudou porque um arquivo de número menor entrou, regrave com `--atualizar` e confira no `git diff` que a diferença é só essa.
- Cada arquivo deixa a sessão `principal` logada, para os seguintes.

### Campos de um caso

```json
{
  "nome": "entrar com senha errada",
  "comentario": "opcional: o porquê do caso",
  "metodo": "POST",
  "caminho": "/api/auth/entrar",
  "sessao": "segundo-ip",
  "corpo": { "login": "ana.teste", "senha": "errada" }
}
```

| Campo | Para quê |
|---|---|
| `nome` | único no arquivo; liga o caso ao esperado |
| `metodo`, `caminho` | a requisição; o caminho aceita `{{variavel}}` |
| `corpo` | enviado como JSON; aceita `{{variavel}}` (entre aspas para texto, sem aspas para número) |
| `corpoTexto` | enviado como está, para JSON quebrado |
| `sessao` | o "navegador" (cookies próprios); vazio é `principal`, a logada |
| `novaSessao` | apaga os cookies da sessão antes (navegador recém-aberto) |
| `semCsrf` | não manda o `X-XSRF-TOKEN` (todo método que não é GET manda, como o Angular) |
| `capturar` | `{"chave": "/resultado/chave"}`: guarda um valor da resposta (JSON Pointer) para os casos seguintes, como a chave do desfazer |
| `normalizar` | `{"/campo": "texto" \| "numero" \| "datahora"}`: troca um campo do ambiente por um marcador, conferindo só o tipo |

No topo do arquivo, `sessoes` dá a uma sessão um IP de origem (`{"segundo-ip": {"origem": "127.0.0.2"}}`). O limite de login é de 5 tentativas por minuto **por IP** (`Program.cs`): para testar login sem cair no 429 sem querer, use outro IP. O `00-auth` testa o 429 de propósito, na 6ª tentativa de `127.0.0.1`.

## O que é comparado

Para cada caso, `esperado/` guarda:
- **status** e **Content-Type** (`application/json` e `application/problem+json` são diferentes);
- **cookies** criados ou apagados (nome e atributos; o valor vira `<valor>`, porque é cifrado);
- **corpo**: JSON comparado por valor, ou o texto, se não for JSON;
- **banco**: as linhas inseridas, alteradas (só as colunas que mudaram, com antes e depois) e removidas em **todas as tabelas**, com o tipo e o texto cru de cada coluna, como o SQLite guarda. É o que garante que o Go grava decimal, data e GUID no formato do EF Core.

Regras da comparação do JSON (`json.go`):
- número compara pelo texto, sem passar por `float64`: `10.50` ≠ `10.5`, `100` ≠ `100.0`. O `System.Text.Json` escreve o decimal com a escala que ele tem, e o Go tem de escrever igual;
- `null` ≠ ausente, `[]` ≠ `null`, número ≠ texto (`5` ≠ `"5"`);
- a ordem dos itens de uma lista importa; a ordem das chaves de um objeto, não (não muda nada para quem lê o JSON).

No banco, INTEGER vira número, TEXT vira texto e NULL vira `null`. REAL e BLOB aparecem como `"real:1.5"` e `"blob:0A0B"`: um REAL no lugar de um decimal é diferença que tem de aparecer.

## Normalizações (o que muda a cada execução)

Só o que muda sozinho a cada execução é trocado por um marcador (`normalizar.go`). **Dinheiro, datas de negócio, ids, mensagens e contagens nunca.**

| O quê | Vira | Por quê |
|---|---|---|
| Data e hora entre o início do relógio e 1 dia depois, no formato do EF Core ou ISO | `<relógio +N min, formato>` | o relógio anda, e cada execução cai noutra fração de segundo. `N` são os minutos desde o início: o bloqueio de login aparece como `+15 min`. O formato fica no marcador (com espaço ou `T`, com ou sem `Z`). Um horário em outro fuso cai fora da janela e aparece cru na diferença |
| GUID | `<GUID>` (maiúsculas, o EF Core) ou `<guid>` (minúsculas, o JSON) | o carimbo de segurança é `Guid.NewGuid()` |
| `traceId` do `ValidationProblemDetails` | `<traceId>` | muda a cada requisição |
| `versao` igual à do `Directory.Build.props` | `<versao>` | sobe a cada PR (ADR 0039) |
| Caminhos dentro da pasta temporária | `<dados>/...` | a pasta muda a cada execução |
| `Usuario.SenhaHash` | `<hash v3: prf 2, 100000 iterações, sal de 16 bytes, chave de 32 bytes>` | o sal é aleatório; o resto tem de ser igual para o .NET ler a senha que o Go gravar |
| Valor dos cookies | `<valor>` | cifrado pelo Data Protection |

**Exceção documentada:** em `GET /api/parametros/sistema`, a versão do runtime (`dotNet`) e do sistema operacional, a hora em que o processo subiu (`iniciadoEm`, que é a do processo e não passa pela libfaketime) e o tamanho do arquivo do banco são do ambiente: o caso confere só o tipo (`normalizar`). O Go vai devolver outro texto em `dotNet`, e isso é esperado.

Os ids do `AUTOINCREMENT` **não** são normalizados: num banco novo eles são sempre os mesmos, e o Go tem de gerar os mesmos.

## O ambiente do .NET no contrato

- **Relógio:** a libfaketime faz o processo começar em **2026-10-15 01:00:00 UTC** e andar a partir dali ("começa em", `FAKETIME="@..."`). Congelar o relógio travaria os temporizadores. O relógio monotônico não é mexido (`FAKETIME_DONT_FAKE_MONOTONIC=1`), então `Task.Delay`, `PeriodicTimer` e o limite de login seguem o tempo real. Em Brasília ainda é **14/10/2026, 22h**: "hoje" é 14/10, e a data UTC é 15/10. Quem usar a data UTC como "hoje" erra, e o contrato pega. O Go, atrás do .NET, recebe o mesmo relógio por `Go:InicioDoRelogio` (#363), porque a libfaketime não age sobre ele.
- **Sem rede:** a API roda num namespace de rede só com o loopback (`unshare -rn`), sem mudar o código. As tarefas do câmbio e da cripto (no Go desde a #81, por `Go:Tarefas`; antes, o `CambioHostedService` e o `CriptoHostedService`) tentam a internet 1 minuto depois de subir; o `base.sql` já marca o câmbio como atualizado hoje, e a cripto falha sem gravar nada além do aviso no log. A sincronização (`SincronizacaoHostedService`, 2 minutos depois) é desligada pelo próprio roteiro em `01-parametros`.
- **Estado inicial:** o de **depois** de subir. A API aplica o esquema e roda as rotinas de partida (`Program.cs`), que acertam dados; o contrato só compara o que cada caso muda.
- **Tabela ignorada:** `ErroSistema`, que recebe os avisos do log em momentos que não dependem dos casos (`TabelasIgnoradas`, em `banco.go`). O aviso do Go de que o relógio de teste está ligado também cai nela.
- **Cultura e fuso:** `pt_BR.UTF-8` e `America/Sao_Paulo`, como no computador de quem usa. As mensagens de validação padrão do ASP.NET (`The Senha field is required.`) saem em inglês nos dois casos: o .NET não traz essas mensagens traduzidas. Conferido rodando com `CULTURA=C.UTF-8`: mesmo resultado.
- **Pasta atual:** a da API compilada, como no serviço instalado, para o `appsettings.json` valer (`AllowedHosts`, logs).

## O que ficou de fora

- **Integrações externas** (Pluggy, Yahoo/brapi, Banco Central, Mercado Bitcoin): fora do contrato de ponta a ponta, porque dependem da internet. São testadas em Go nas Etapas 11 a 13, com respostas gravadas.
- **Cabeçalhos de segurança, cache e arquivos do front-end:** conferidos contra o .NET rodando por `internal/servidor/igual_dotnet_test.go`.
- **O que só existe no Windows** (DPAPI, serviço, instalador): continua com o teste do `.msi`.
- **Relação entre GUIDs:** todo GUID vira o mesmo marcador, então o contrato não confere que duas transações têm o mesmo `TransferenciaGrupo` pelo valor (confere pela contagem e pelos outros campos).
