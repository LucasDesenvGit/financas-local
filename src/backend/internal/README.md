# O padrão de um assunto em Go

Cada assunto é um pacote em `internal/<area>/<assunto>`, com HTTP, regra e banco dentro ([ADR 0056](../../../docs/adr/0056-arquitetura-do-codigo-go-camadas-por-assunto-e-ddd-estrategico.md)); o mapa do back-end está no [docs/ARQUITETURA.md](../../../docs/ARQUITETURA.md#back-end--srcbackend). O modelo é o `internal/ajustes/categorias`, com as regras em `internal/ajustes/categorizacao`, `regrasdescricao` e `internal/movimentacao/transferencias`.

Como criar ou mudar um assunto (rotas, entrada e saída, gravar, testes e as listas que a CI cobra) está na skill [`go-no-projeto`](../../../.claude/skills/go-no-projeto/SKILL.md). O teste de arquitetura local confere as fronteiras (`go test -tags arquitetura ./internal/arquitetura/`, #476).

## As áreas

As áreas são as mesmas das telas do front-end (`src/frontend/src/app/paginas/`), para a mesma palavra levar à mesma pasta nos dois lados (#656). A área só organiza: o pacote continua com o nome do assunto (`transacoes.NovoServico`), e quem importa quem fica na tabela `dependencias` do teste de arquitetura.

| Pasta | O que tem | Pacotes |
|---|---|---|
| `movimentacao/` | o dinheiro que entra e sai: transações, contas, cartões, contas a pagar | `transacoes`, `extrato`, `transferencias`, `parcelas`, `faturas`, `contas`, `contasagendadas`, `cobrancas`, `conferenciabanco`, `sugestoes`, `bancos`, `tipolancamento`, `poucoinformativa` |
| `planejamento/` | orçamentos e metas | `orcamentos`, `metas` |
| `patrimonio/` | investimentos, reservas, empréstimos e as cotações | `carteira`, `investimentos`, `reservas`, `cambio`, `cotacoes`, `cripto`, `selic`, `emprestimos` |
| `analises/` | relatórios e painel | `relatorios`, `painel` |
| `ajustes/` | os cadastros e a parametrização | `categorias`, `categorizacao`, `regrasdescricao`, `parametros` |
| `openfinance/` | a Pluggy e a sincronização (no front-end, a tela fica em `ajustes`) | `openfinance`, `pluggy`, `sincronizacao` |
| `plataforma/` | o que todos usam: banco, tipos, HTTP comum, login, erros do sistema, auditoria, segredos, tarefas | `banco`, `tipos`, `texto`, `relogio`, `config`, `respostas`, `servidor`, `servicosistema`, `sessao`, `autenticacao`, `auditoria`, `configuracao`, `erros`, `segredos`, `tarefas`, `versao`, `saude`, `desfazer`, `celular`, `modulos`, `testeapoio` |
| `modulos/` | o que fica fora do núcleo financeiro, ligado no banco ([ADR 0064](../../../docs/adr/0064-modulos-compilados-no-executavel-e-ligados-no-banco.md)) | `noticias`, `trabalho` |
| raiz de `internal/` | quem junta ou confere todos os outros | `api`, `partida`, `arquitetura` |

**Assunto novo:** vai para a área da tela dele. Se ninguém o vê como tela e todos os assuntos o usam, é `plataforma`. Em dúvida, a ADR 0056 decide pelo mapa de contextos, não pelo nome.
