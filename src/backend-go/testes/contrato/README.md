# Testes de contrato

A rede de segurança da migração para Go ([#72](https://github.com/LucasDesenvGit/financas-local/issues/72), [ADR 0049](../../../../docs/adr/0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md)). O que o back-end .NET responde hoje fica gravado em `esperado/` e vira a régua do Go: **mesmo status, mesmo JSON e o banco no mesmo estado**, caso a caso.

São *characterization tests*: não dizem o que o sistema deveria fazer, congelam o que ele faz. Se o .NET tem um comportamento estranho, o esperado também tem, de propósito. Diferença de comportamento vira issue, não correção no contrato.

| Pasta ou arquivo | O que tem |
|---|---|
| `casos/NN-assunto.json` | o roteiro: requisições em ordem, um arquivo por assunto |
| `esperado/NN-assunto.json` | o que o .NET respondeu e gravou em cada caso (*golden files*), já normalizado |
| `dados/base.sql` | o banco fictício: dados **inventados**, nunca de uma pessoa de verdade |
| `rodar-dotnet.sh` | monta o banco, sobe o .NET sem rede e com o relógio controlado, e roda o comparador |
| `*.go` | o gravador e o comparador (Go puro; `go test ./...` roda os testes deles sem servidor) |

Hoje: `00-auth` (40 casos) e `01-parametros` (14 casos). **Cada etapa do épico grava os casos do próprio assunto, no próprio PR, antes do código Go** (ADR 0049).

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

Cada execução começa do zero: banco novo, processo novo. O roteiro depende da ordem (o `00-auth` cria o usuário e deixa a sessão `principal` logada para os seguintes), então ele sempre roda inteiro.

## Rodar contra o Go (próximas etapas)

O comparador é o mesmo; muda o servidor:

```bash
cd src/backend-go
go test ./testes/contrato -count=1 -args \
  -alvo=https://127.0.0.1:5444 -banco=/tmp/contrato/dados/sql/FinancasLocal.db -inicio=2026-10-15T01:00:00Z
```

- O servidor Go tem de usar um banco montado como o do `rodar-dotnet.sh` (estrutura, colunas, dados iniciais, `base.sql`), no mesmo estado em que o .NET fica depois de subir, e ter o relógio começando em `-inicio`. A libfaketime não age sobre o Go, que lê o relógio sem passar pela libc: o Go vai precisar de uma opção de teste para o relógio injetável (`internal/relogio`).
- **Nunca grave os esperados a partir do Go.** O `-atualizar` é só contra o .NET: ele é a referência.

## Adicionar um caso

1. Escreva o caso no `casos/NN-assunto.json` do assunto (ou crie o arquivo; o número dá a ordem).
2. Grave os esperados: `rodar-dotnet.sh --atualizar`.
3. Leia a diferença de `esperado/` no `git diff`: é a descrição exata do que o .NET faz. Confira que nenhum esperado de outro assunto mudou sem motivo.
4. Rode de novo **sem** `--atualizar`, duas vezes: tem de passar as duas.

Cubra os erros também (400, 401, 404, 409, 429), não só o caminho feliz. Teste novo que passa uma vez e falha na outra é instável: descubra o que muda e normalize (ou mude o caso), nunca afrouxe a comparação.

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

- **Relógio:** a libfaketime faz o processo começar em **2026-10-15 01:00:00 UTC** e andar a partir dali ("começa em", `FAKETIME="@..."`). Congelar o relógio travaria os temporizadores. O relógio monotônico não é mexido (`FAKETIME_DONT_FAKE_MONOTONIC=1`), então `Task.Delay`, `PeriodicTimer` e o limite de login seguem o tempo real. Em Brasília ainda é **14/10/2026, 22h**: "hoje" é 14/10, e a data UTC é 15/10. Quem usar a data UTC como "hoje" erra, e o contrato pega.
- **Sem rede:** a API roda num namespace de rede só com o loopback (`unshare -rn`), sem mudar o código. O câmbio e a cripto (`CambioHostedService`, `CriptoHostedService`) tentam a internet 1 minuto depois de subir; o `base.sql` já marca o câmbio como atualizado hoje, e a cripto falha sem gravar nada além do aviso no log. A sincronização (`SincronizacaoHostedService`, 2 minutos depois) é desligada pelo próprio roteiro em `01-parametros`.
- **Estado inicial:** o de **depois** de subir. A API aplica o esquema e roda as rotinas de partida (`Program.cs`), que acertam dados; o contrato só compara o que cada caso muda.
- **Tabela ignorada:** `ErroSistema`, que recebe os avisos do log em momentos que não dependem dos casos (`TabelasIgnoradas`, em `banco.go`).
- **Cultura e fuso:** `pt_BR.UTF-8` e `America/Sao_Paulo`, como no computador de quem usa. As mensagens de validação padrão do ASP.NET (`The Senha field is required.`) saem em inglês nos dois casos: o .NET não traz essas mensagens traduzidas. Conferido rodando com `CULTURA=C.UTF-8`: mesmo resultado.
- **Pasta atual:** a da API compilada, como no serviço instalado, para o `appsettings.json` valer (`AllowedHosts`, logs).

## O que ficou de fora

- **Integrações externas** (Pluggy, Yahoo/brapi, Banco Central, Mercado Bitcoin): fora do contrato de ponta a ponta, porque dependem da internet. São testadas em Go nas Etapas 11 a 13, com respostas gravadas.
- **Cabeçalhos de segurança, cache e arquivos do front-end:** conferidos contra o .NET rodando por `internal/servidor/igual_dotnet_test.go`.
- **O que só existe no Windows** (DPAPI, serviço, instalador): continua com o teste do `.msi`.
- **Relação entre GUIDs:** todo GUID vira o mesmo marcador, então o contrato não confere que duas transações têm o mesmo `TransferenciaGrupo` pelo valor (confere pela contagem e pelos outros campos).
