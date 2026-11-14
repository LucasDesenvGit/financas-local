# 0064. Módulos compilados no executável, ligados e desligados no banco (micro-kernel)

- **Status:** Proposta
- **Data:** 2026-11-13
- **Relacionadas:** [0015](0015-parametros-do-sistema-no-banco-editaveis-pelo-site.md), [0017](0017-preferencias-de-exibicao-no-navegador.md), [0026](0026-manual-do-site-como-dados-no-front-end.md), [0032](0032-controles-padrao-interruptor-e-campo-de-valor.md), [0046](0046-menu-lateral-em-grupos-que-abrem-e-fecham.md), [0056](0056-arquitetura-do-codigo-go-camadas-por-assunto-e-ddd-estrategico.md), [0058](0058-pastas-do-front-end-por-grupo-do-menu-e-por-assunto.md)

## Contexto

O site é de finanças. Algumas visualizações vão além do básico: o Trabalho (salário, holerite, férias, #209) e as Notícias (#10), e outras depois. Quem não usa não deve ver o menu e o Painel mais pesados (Nielsen 8, *Aesthetic and Minimalist Design*), e uma visualização que acessa a internet não deve acessar nada enquanto a pessoa não a quer (privacidade; #68, item 11). O épico #190 pede um jeito único de uma visualização entrar no site, aparecer ou sumir e parar de rodar quando desligada, sem mexer no núcleo a cada visualização nova.

Restrições: tudo roda num executável só, como serviço do sistema, em Windows e Linux (ADR 0049, #86); a CSP proíbe carregar código de fora no navegador (`docs/SEGURANCA.md`); o liga/desliga precisa valer também para as tarefas em segundo plano, que não passam pelo navegador.

O estilo é o **micro-kernel** (Richards e Ford, *Fundamentals of Software Architecture*, 2020): um núcleo (as finanças) e módulos que se ligam a ele por um contrato.

## Alternativas consideradas

- **Módulo carregado do disco** (pacote [`plugin`](https://pkg.go.dev/plugin) do Go): não funciona no Windows (só Linux, FreeBSD e macOS); exige o mesmo Go e as mesmas versões das dependências no núcleo e no plugin; e carregar código de uma pasta é a ameaça da #68 (outro processo do computador trocando o que o serviço executa). Rejeitada.
- **Módulo como processo separado:** cada módulo seria um executável com a própria porta. Repete o custo que a #86 tirou (mais processos, segredo entre eles, repasse) e abre fronteiras novas no modelo de ameaças. Rejeitada.
- **Liga/desliga só no front-end** (preferência no navegador, ADR 0017): o menu esconderia a tela, mas a API e as tarefas de fundo continuariam rodando e acessando a internet, e cada navegador teria a sua escolha. Rejeitada: o desligado tem de valer para o servidor.
- **Módulo compilado no executável, registrado no núcleo e ligado ou desligado no banco** (escolhida).

## Decisão

**Um módulo é código compilado junto com o site**, que implementa um contrato do núcleo e se liga ou desliga em **Configurações › Módulos** enquanto o site roda. A escolha fica no **banco** (tabela `Configuracao`, chave `Modulos.<id>.Ligado`), como os parâmetros (ADR 0015), porque vale para qualquer navegador e para as tarefas de fundo.

### O contrato (Go, `internal/plataforma/modulos`)

```go
type Modulo interface {
	Info() Info                 // id, nome, descrição, para onde acessa a internet, se nasce ligado
	Registrar(rotas *Rotas)     // as rotas, todas em /api/<id>/...
	Tarefas() []Tarefa          // as tarefas em segundo plano (pode ser nenhuma)
}
```

- **O núcleo guarda as rotas:** o módulo registra pelo `Rotas`, e não direto no `ServeMux`. Toda rota do módulo passa por uma checagem: desligado, a API responde **409** com "O módulo X está desligado. Ligue em Configurações › Módulos." e não roda nada do módulo. O `Rotas` só aceita caminho em `/api/<id>/`, para o módulo não esconder rota fora do prefixo dele (a montagem falha com a rota errada).
- **O núcleo roda as tarefas:** cada `Tarefa` (nome, atraso, intervalo e o ciclo) roda pelo `tarefas.Periodica`, e cada ciclo pergunta antes se o módulo está ligado. Desligado, o ciclo não roda, e não sai nada para a internet.
- **Padrão:** módulo que acessa a internet nasce **desligado** (a montagem falha se não). *Inferido:* os outros também nascem desligados, salvo motivo, para a versão nova não encher o menu de quem não pediu (Nielsen 8).
- **Auditoria:** ligar e desligar fica em "Acessos recentes" (`ModuloLigado (<id>)`, `ModuloDesligado (<id>)`).
- **Onde mora:** `internal/plataforma/modulos` é o núcleo dos módulos (catálogo, `Rotas`, tarefas, as rotas `GET /api/modulos` e `PUT /api/modulos/{id}`). Cada módulo fica em `internal/modulos/<id>` (e os assuntos dele em `internal/modulos/<id>/<assunto>`, no mesmo padrão de pacote por assunto da ADR 0056). O módulo usa o núcleo; o núcleo não importa módulo; um módulo não importa outro (o teste de arquitetura confere pela raiz `internal/modulos/<id>`). Quem monta é o `internal/api`.

### No front-end

- `ModulosService` (`core/modulos.service.ts`) lê `GET /api/modulos` uma vez, guarda num signal e o atualiza quando a pessoa liga ou desliga: o menu e as telas mudam **sem recarregar a página**.
- **Menu:** a tela ou o grupo do módulo leva `modulo: '<id>'` no `MENU`; o `filtrarMenu` (#227, ADR 0046) já tira a tela e o grupo que fica vazio.
- **Rota:** a rota do módulo fica dentro do `PortaoModuloComponent` (`compartilhado/modulos/`), que mostra a tela só com o módulo ligado. Desligado, mostra "Este módulo está desligado", o que ele faz e o que acessa na internet, com o botão para ligar ali mesmo e o link para Configurações › Módulos. Como a tela nem é criada, ela não chama a API.
- **Painel:** um card de módulo pergunta ao `ModulosService` se ele está ligado (nenhum módulo tem card ainda).
- **Código:** a tela do módulo é compilada no build, com `loadComponent` (só baixa quando abre). Carregar código de fora no navegador a CSP já proíbe.
- **Configurações › Módulos:** um interruptor por módulo, com efeito imediato (ADR 0032; Kendrick, NN/g, *Toggle-Switch Guidelines*), o nome, a descrição e, se acessa a internet, para onde.

### O que não vira módulo

O núcleo financeiro: transações, contas, cartões, contas a pagar, orçamentos, metas, categorização, relatórios. *Julgamento:* investimentos também, porque usa contas e transações (extrato da carteira, proventos).

## Consequências

- **Positivas:**
  - Visualização nova entra sem mexer no núcleo: um pacote, uma linha no `internal/api`, uma entrada no `MENU` e a seção do Manual.
  - Desligado vale de verdade: sem rota, sem tarefa, sem saída de rede, em qualquer navegador.
  - Um executável só, sem pasta de plugins para proteger nem processo a mais.
- **Negativas:**
  - Módulo novo exige uma versão nova do site (não dá para instalar um módulo à parte). Aceito: é assim com todo o resto.
  - O código de um módulo desligado continua no executável e no `wwwroot` (só não roda). O tamanho cresce um pouco a cada módulo.
  - As tabelas de um módulo existem no banco mesmo com ele desligado (o `01-estrutura.sql` é um só). Desligar não apaga os dados: ligar de novo traz tudo de volta.
  - Cada ciclo de tarefa de módulo lê a `Configuracao` antes de rodar (uma leitura pequena).
- **Na prática (checklist do módulo novo, épico #190):**
  - pacote em `internal/modulos/<id>` que implementa `modulos.Modulo`, montado no `internal/api` (`catalogo.Adicionar`), e o caminho no grupo `modulos` do teste de arquitetura;
  - rotas só em `/api/<id>/`, pelo `Rotas`; tarefas pelo `Tarefas()`, nunca uma goroutine própria;
  - acessa a internet: `Destinos` preenchido, nasce desligado, ADR da saída de rede e a tabela "O que sai para a internet" (`docs/SEGURANCA.md`, README);
  - front-end: `modulo: '<id>'` no `MENU` e a rota dentro do `PortaoModuloComponent`;
  - Manual: a seção do módulo diz como ligar;
  - testes: desligado, a rota da API responde 409 e a tarefa não roda; no navegador, some do menu e a rota mostra o aviso.
