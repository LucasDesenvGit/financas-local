# 0023. Edição de categorias em massa como "modo edição" de Transações, com desfazer guardado na API

- **Status:** Aceita
- **Data:** 2026-10-17 (PR da issue #87)
- **Relacionadas:** [0018](0018-estado-da-tela-na-url-e-detalhe-por-navegacao.md) (filtros e estado na URL), [0021](0021-componentes-padrao-para-graficos-e-telas-densas.md) (componentes padrão)

## Contexto

Arrumar a categoria de muitas transações exigia abrir o formulário de cada uma. A categorização em lote de Transações só valia para as linhas selecionadas da página e gravava na hora, sem desfazer. A issue #87 pedia:
- trocar na linha, com alteração pendente, "Salvar alterações (N)" e "Descartar";
- trocar em massa, inclusive "todas do filtro", com confirmação;
- "Desfazer" depois de salvar;
- a API garantindo que só a categoria muda.

A issue deixou em aberto se isso seria uma **rota nova** ou um **modo edição** dentro de Transações. Os filtros de Transações (período, conta, várias categorias, tipo, emergências, busca, chips de filtro ativo, URL, paginação) já existem e têm testes.

## Alternativas consideradas

- **Rota nova (`/transacoes/gerenciar`).**
  - Prós: tela mais simples, cada uma com um propósito.
  - Contras:
    - duplicaria filtros, chips, leitura e gravação da URL e paginação, ou exigiria extraí-los antes para componentes compartilhados (refatoração grande na tela mais usada);
    - seriam duas telas quase iguais para manter;
    - ficaria a dúvida sobre onde ficam as ações em lote atuais.
- **Modo edição dentro de Transações (`?editar=1`).**
  - Prós:
    - reaproveita tudo o que já existe, sem duplicar;
    - quem já filtrou a lista só liga a edição;
    - as ações em lote atuais continuam no mesmo lugar.
  - Contras: a tela de Transações fica maior.
- **Desfazer com o estado anterior devolvido ao navegador.**
  - Prós: a API não guarda nada.
  - Contras: o navegador mandaria de volta tipo e "ignorar nos relatórios", o que permitiria alterar por esse endpoint campos que não estão liberados.
- **Desfazer com o estado anterior guardado na API.**
  - Prós: o navegador só recebe uma chave e não tem como mandar outros campos.
  - Contras: o desfazer se perde se a API reiniciar.

## Decisão

A edição em massa é um **modo edição** da própria tela de Transações, ligado pelo botão "Editar categorias" e gravado na URL (`?editar=1`, ADR 0018).
- **Na linha:** cada transação ganha um seletor de categoria. A troca fica **pendente** (linha destacada, com a etiqueta "alterada") até **"Salvar alterações (N)"**. Há também "Descartar", e Esc numa linha desfaz a troca dela. Trocar de página ou de filtro, ou sair do modo, com trocas pendentes pede confirmação.
- **Em massa:** a barra de seleção marca a troca nas linhas selecionadas, como pendência. "Todas as N do filtro" grava de uma vez, depois de uma confirmação com a quantidade, o período e a categoria.
- **Na API:** um endpoint só, `PATCH /api/transacoes/em-lote`.
  - Recebe por transação (`itens`, até 1000) ou pelo mesmo filtro da busca (`filtro`, até 5000).
  - Muda só a categoria, com as mesmas regras do PATCH de uma transação.
  - O estado anterior (categoria, manual, tipo e "ignorar nos relatórios") fica no `IMemoryCache` por 10 minutos, e a resposta traz só a chave.
  - `POST /api/transacoes/em-lote/desfazer/{chave}` restaura esse estado, mas não passa por cima de uma transação que mudou de novo depois.
  - As duas operações vão para a auditoria (quantidade, sem valores).
- **Depois de salvar:** um aviso com "Desfazer". O `AvisosService` ganhou a ação opcional e pausa o tempo com mouse ou foco no aviso. Se várias transações iguais foram para a mesma categoria, a tela oferece "Criar regra para as próximas".

## Consequências

- **Positivas:**
  - categorizar dezenas de transações virou escolher e salvar uma vez;
  - nenhum filtro foi duplicado;
  - o desfazer não abre brecha para alterar outros campos;
  - as regras nascem do próprio uso.
- **Negativas:**
  - `transacoes.component.ts` ficou maior: o modo edição vive no mesmo componente. Se crescer mais (ex.: emergência e "ignorar" na linha), vale extraí-lo.
  - O "Desfazer" vale por 10 minutos e se perde se a API reiniciar.
  - "Todas do filtro" tem limite de 5000 por vez; acima disso, é preciso restringir o período.
  - As setas ↑/↓ não navegam entre linhas, porque num `<select>` fechado elas trocam a opção. A navegação é por Tab (julgamento próprio, ainda não validado com uso real).
- **Na prática:**
  - campo novo editável em massa entra no mesmo `PATCH em-lote` (DTO e `EstadoAnterior`), nunca num endpoint por campo;
  - a consulta da lista e a do "todas do filtro" são a mesma (`TransacoesController.Filtrar`): filtro novo de Transações vale para os dois.
