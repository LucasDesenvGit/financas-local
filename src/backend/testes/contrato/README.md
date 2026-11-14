# Testes de contrato

A rede de segurança do back-end ([#72](https://github.com/LucasDesenvGit/financas-local/issues/72), [ADR 0049](../../../../docs/adr/0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md), [ADR 0051](../../../../docs/adr/0051-testes-do-back-end-em-go-cada-regra-com-teste.md)). Cada caso confere **o status, o JSON e o banco no estado esperado**.

**Da migração à regressão (#86):** os esperados (`esperado/`) foram gravados do back-end .NET, caso a caso, e foram a régua do Go durante a migração. Com o .NET fora, eles são a **regressão do Go**: congelam o que o site faz hoje. São *characterization tests*: não dizem o que o sistema deveria fazer, congelam o que ele faz. Um esperado só muda com o motivo escrito no PR (veja [Mudar um esperado](#mudar-um-esperado)).

| Pasta ou arquivo | O que tem |
|---|---|
| `casos/NN-assunto.json` | o roteiro: requisições em ordem, um arquivo por assunto |
| `esperado/NN-assunto.json` | o que o site respondeu e gravou em cada caso (*golden files*), já normalizado |
| `dados/base.sql` | o banco fictício: dados **inventados**, nunca de uma pessoa de verdade |
| `dados/pluggy/` | as respostas fictícias da Pluggy falsa (`pluggyfalsa`) |
| `rodar.sh` | monta o banco, sobe o Go como o serviço instalado, sem rede e com o relógio controlado, e roda o comparador |
| `*.go` | o gravador e o comparador (Go puro; `go test ./...` roda os testes deles sem servidor) |

Hoje: `00-auth` (40 casos), `01-parametros` (14), `02-cambio` (15), `03-cripto` (2), `04-contas` (65), `05-categorias` (33), `06-regras` (38), `07-transacoes` (205), `08-orcamentos` (41), `09-metas` (49), `10-contas-agendadas` (44), `11-investimentos` (134), `12-conferencia-investimentos` (42), `13-painel` (10), `14-relatorios` (29), `15-emprestimos` (2), `16-open-finance` (64, #83), `17-erros` (52) e `90-go` (4, o diagnóstico `/api/go/saude`).

## Rodar

Precisa do Go e do `sqlite3`. No Linux, de preferência também do `unshare` (util-linux).

```bash
src/backend/testes/contrato/rodar.sh                                   # compara com esperado/
src/backend/testes/contrato/rodar.sh --atualizar                       # grava esperado/ (confira a diferença)
CONTRATO_SOMENTE="00-auth,01-parametros" src/backend/testes/contrato/rodar.sh
```

- O roteiro compila o Go (`cmd/financas`, sem cgo e com a versão do `Directory.Build.props`, como na publicação), o comparador e a Pluggy falsa, e sobe o Go com a linha do serviço instalado (`-aplicar-esquema -rotinas-de-partida`), na porta `5451` (`PORTA=...` para trocar).
- `MANTER_PASTA=1` guarda a pasta temporária, com o banco e o log do Go (`go.log`).
- **Roda também no Git Bash do Windows** (conferido em 03/11/2026, com todos os arquivos).
- **Ubuntu 24.04:** o `unshare -rn` precisa de `sudo sysctl -w kernel.apparmor_restrict_unprivileged_userns=0` (a CI faz isso). Sem ele, o roteiro usa o proxy fechado (abaixo).

Cada execução começa do zero: banco novo, processo novo. O roteiro depende da ordem (o `00-auth` cria o usuário e deixa a sessão `principal` logada para os seguintes), então o padrão é rodar inteiro. Com `CONTRATO_SOMENTE`, mantenha a ordem e os arquivos de que o seu depende.

À mão, sem o roteiro:
- gere o certificado: `echo contrato | go run ./cmd/financas gerar-certificado -pfx <pasta>/financaslocal.pfx`, que grava também o `<pasta>/financaslocal.pem`, e ponha o caminho e a senha no `appsettings.Production.json` da pasta do `-pasta`, como o `rodar.sh` faz;
- suba o Go: `go run ./cmd/financas -pasta <pasta> -endereco 127.0.0.1:5453 -wwwroot internal/plataforma/servidor/testdata/wwwroot -banco <db> -inicio-do-relogio 2026-10-15T01:00:00Z -aplicar-esquema -rotinas-de-partida`, com `HTTPS_PROXY=http://127.0.0.1:9`;
- rode `go test ./testes/contrato -count=1 -args -alvo=https://127.0.0.1:5453 -raiz=<pasta>/financaslocal.pem -banco=<db> -inicio=2026-10-15T01:00:00Z`.

O cliente confere o certificado do Go e confia só no `.pem` do `-raiz` (#638): sem `InsecureSkipVerify`, o que o CodeQL aponta. O `-inicio` do comparador e o `-inicio-do-relogio` do Go são o mesmo instante.

## Adicionar um caso

1. Escreva o caso no `casos/NN-assunto.json` do assunto (ou crie o arquivo, com o número da [tabela](#numeração-dos-arquivos-de-casos); o número dá a ordem).
2. Grave os esperados: `rodar.sh --atualizar`.
3. Leia a diferença de `esperado/` no `git diff`: é a descrição exata do que o site faz. Confira que **só** o caso novo apareceu, e que nenhum esperado de outro assunto mudou.
4. Rode de novo **sem** `--atualizar`, duas vezes: tem de passar as duas.

Cubra os erros também (400, 401, 404, 409, 429), não só o caminho feliz. Teste novo que passa uma vez e falha na outra é instável: descubra o que muda e normalize (ou mude o caso), nunca afrouxe a comparação.

## Mudar um esperado

Um esperado que já existe muda só de propósito (ADR 0051):
- **Correção de bug** ou **mudança pedida**: o PR diz, para cada esperado que mudou, o porquê, e o `git diff` de `esperado/` mostra só isso.
- **Diferença que apareceu sem querer** é regressão: o código volta, e o esperado fica.

Exemplo: na #86, o `90-go` "POST sem token CSRF" passou de 400 para 404. O 400 vinha do .NET na frente, que conferia o CSRF antes do repasse; o Go confere o CSRF só nas rotas que existem, e esta só tem GET. O CSRF do Go continua conferido pelos casos de `00-auth`, `01-parametros` e `17-erros`.

### Numeração dos arquivos de casos

O número dá a ordem dos dados: o painel lê o que as transações gravam, e não o contrário. Funcionalidade nova entra num número livre entre `20` e `89`, por ordem de chegada.

| Arquivo | Assunto | Issue | Prefixos |
|---|---|---|---|
| `00-auth` | login, sessão, CSRF, limites | #75 | `/api/auth` |
| `01-parametros` | parâmetros e configurações | #76 | `/api/parametros` |
| `02-cambio` | câmbio (PTAX) | #81 | `/api/cambio` |
| `03-cripto` | cripto | #81 | `/api/cripto` |
| `04-contas` | contas e conferência do banco | #77 (contas), #445 (conferência) | `/api/contas`, `/api/conferencia-banco` |
| `05-categorias` | categorias | #77 | `/api/categorias` |
| `06-regras` | regras e regras de descrição | #77 | `/api/regras`, `/api/regras-descricao` |
| `07-transacoes` | transações | #78 | `/api/transacoes` |
| `08-orcamentos` | orçamentos | #79 | `/api/orcamentos` |
| `09-metas` | metas | #79 | `/api/metas` |
| `10-contas-agendadas` | contas a pagar | #79 | `/api/contas-agendadas` |
| `11-investimentos` | carteira, reservas, investimentos | #82, #80 | `/api/carteira`, `/api/reservas`, `/api/investimentos` |
| `12-conferencia-investimentos` | conferir investimentos | #299 | `/api/carteira/conferencia` |
| `13-painel` | painel | #80 | `/api/painel` |
| `14-relatorios` | relatórios | #80 | `/api/relatorios` |
| `15-emprestimos` | empréstimos | #80 | `/api/emprestimos` |
| `16-open-finance` | Open Finance (Pluggy) | #83 | `/api/open-finance` |
| `17-erros` | erros do sistema | #298 | `/api/erros` |
| `20-modulos` | módulos que se ligam e desligam | #191 | `/api/modulos` |
| `21-trabalho` | módulo Trabalho: emprego, salário, estimativa e tabelas | #210 | `/api/trabalho` |
| `22-holerite` | holerite do mês do Trabalho | #211 | `/api/trabalho/holerites` |
| `23-decimo-terceiro` | 13º salário do Trabalho | #214 | `/api/trabalho/empregos/{id}/decimo-terceiro` |
| `24-ferias` | férias do Trabalho | #213 | `/api/trabalho/empregos/{id}/ferias` |
| `25-fgts` | FGTS do Trabalho | #216 | `/api/trabalho/empregos/{id}/fgts` |
| `26-recebimento` | recebimento do salário (Trabalho) | #212 | `/api/trabalho/holerites/{id}/recebimentos` |
| `27-rescisao` | rescisão do Trabalho | #215 | `/api/trabalho/empregos/{id}/rescisao` |
| `28` a `89` | funcionalidade nova, por ordem de chegada | — | os dela |
| `90-go` | diagnóstico (`/api/go/saude`) | #231, #363 | `/api/go` |

- Um arquivo pode usar o `base.sql`, a sessão `principal` do `00-auth` e o que um arquivo de número menor **já mesclado** deixou no banco; nunca o de um assunto ainda em andamento.
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

No topo do arquivo, `sessoes` dá a uma sessão um IP de origem (`{"segundo-ip": {"origem": "127.0.0.2"}}`). O limite de login é de 5 tentativas por minuto **por IP**: para testar login sem cair no 429 sem querer, use outro IP. O `00-auth` testa o 429 de propósito, na 6ª tentativa de `127.0.0.1`.

## O que é comparado

Para cada caso, `esperado/` guarda:
- **status** e **Content-Type** (`application/json` e `application/problem+json` são diferentes);
- **cookies** criados ou apagados (nome e atributos; o valor vira `<valor>`, porque é cifrado);
- **corpo**: JSON comparado por valor, ou o texto, se não for JSON;
- **banco**: as linhas inseridas, alteradas (só as colunas que mudaram, com antes e depois) e removidas em **todas as tabelas**, com o tipo e o texto cru de cada coluna, como o SQLite guarda. É o que garante que decimal, data e GUID continuam no formato do EF Core (ADR 0049), o de todos os bancos que já existem.

Regras da comparação do JSON (`json.go`):
- número compara pelo texto, sem passar por `float64`: `10.50` ≠ `10.5`, `100` ≠ `100.0`;
- `null` ≠ ausente, `[]` ≠ `null`, número ≠ texto (`5` ≠ `"5"`);
- a ordem dos itens de uma lista importa; a ordem das chaves de um objeto, não (não muda nada para quem lê o JSON).

No banco, INTEGER vira número, TEXT vira texto e NULL vira `null`. REAL e BLOB aparecem como `"real:1.5"` e `"blob:0A0B"`: um REAL no lugar de um decimal é diferença que tem de aparecer.

## Normalizações (o que muda a cada execução)

Só o que muda sozinho a cada execução é trocado por um marcador (`normalizar.go`). **Dinheiro, datas de negócio, ids, mensagens e contagens nunca.**

| O quê | Vira | Por quê |
|---|---|---|
| Data e hora entre o início do relógio e 1 dia depois, no formato do EF Core ou ISO | `<relógio +N min, formato>` | o relógio anda, e cada execução cai noutra fração de segundo. `N` são os minutos desde o início: o bloqueio de login aparece como `+15 min`. O formato fica no marcador (com espaço ou `T`, com ou sem `Z`). Um horário em outro fuso cai fora da janela e aparece cru na diferença |
| GUID | `<GUID>` (maiúsculas, o EF Core) ou `<guid>` (minúsculas, o JSON) | o carimbo de segurança é aleatório |
| `traceId` do `ValidationProblemDetails` | `<traceId>` | muda a cada requisição |
| `versao` igual à do `Directory.Build.props` | `<versao>` | sobe a cada PR (ADR 0039) |
| Caminhos dentro da pasta temporária | `<dados>/...` | a pasta muda a cada execução |
| `Usuario.SenhaHash` | `<hash v3: prf 2, 100000 iterações, sal de 16 bytes, chave de 32 bytes>` | o sal é aleatório; o resto tem de ser igual, para a senha de quem já tem conta continuar valendo |
| Valor dos cookies | `<valor>` | cifrado |
| `Configuracao.Valor` cifrado | `<segredo cifrado>` | cada gravação cifra diferente |

**Exceção documentada:** em `GET /api/parametros/sistema`, a versão do runtime (`dotNet`) e do sistema operacional, a hora em que o processo subiu (`iniciadoEm`) e o tamanho do arquivo do banco são do ambiente: o caso confere só o tipo (`normalizar`).

Os ids do `AUTOINCREMENT` **não** são normalizados: num banco novo eles são sempre os mesmos.

## O ambiente no contrato

- **Relógio:** o Go começa em **2026-10-15 01:00:00 UTC** (`-inicio-do-relogio`) e anda a partir dali (`relogio.APartirDe`). Em Brasília ainda é **14/10/2026, 22h**: "hoje" é 14/10, e a data UTC é 15/10. Quem usar a data UTC como "hoje" erra, e o contrato pega. (Até a #86, o .NET usava a libfaketime e passava o mesmo instante ao Go por `Go:InicioDoRelogio`, #363.)
- **Sem rede:** no Linux, o Go roda num namespace de rede só com o loopback (`unshare -rn`). Sem o namespace, o roteiro sobe o Go com `HTTPS_PROXY=http://127.0.0.1:9`, uma porta fechada: o `http.Client` do Go segue o proxy ([`ProxyFromEnvironment`](https://pkg.go.dev/net/http#ProxyFromEnvironment)), e o loopback (o teste e a Pluggy falsa) nunca passa por ele. As tarefas do câmbio e da cripto tentam a internet 1 minuto depois de subir; o `base.sql` já marca o câmbio como atualizado hoje, e a cripto falha sem gravar nada além do aviso no log (#416). A sincronização automática é desligada pelo próprio roteiro em `01-parametros`.
- **Pluggy falsa:** `pluggyfalsa`, em `127.0.0.1:5452`, com um certificado próprio. O Go a usa por `Pluggy__UrlBase` e confia nela por `SSL_CERT_FILE`.
- **Estado inicial:** o de **depois** de subir. O Go aplica o esquema e roda as rotinas de partida (#444), que acertam dados; o contrato só compara o que cada caso muda.
- **Tabela ignorada:** `ErroSistema`, que recebe os avisos do log em momentos que não dependem dos casos (`TabelasIgnoradas`, em `banco.go`). O aviso do Go de que o relógio de teste está ligado também cai nela.

## O que ficou de fora

- **Integrações externas** (Yahoo/brapi, Banco Central, Mercado Bitcoin): fora do contrato de ponta a ponta, porque dependem da internet. São testadas em Go, nos pacotes delas, com respostas gravadas. A **Pluggy** entra pela Pluggy falsa.
- **Cabeçalhos de segurança, cache e arquivos do front-end:** testes do pacote `internal/plataforma/servidor` (`TestCabecalhosDeSeguranca`, `TestRespostasComoODotnet`).
- **O que só existe no Windows** (serviço, instalador): continua com o teste do `.msi`.
- **Relação entre GUIDs:** todo GUID vira o mesmo marcador, então o contrato não confere que duas transações têm o mesmo `TransferenciaGrupo` pelo valor (confere pela contagem e pelos outros campos).
