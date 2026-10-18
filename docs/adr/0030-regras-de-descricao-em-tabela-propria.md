# 0030. Regras de descrição em tabela própria, com o nome do banco sempre guardado

- **Status:** Aceita
- **Data:** 2026-10-18 (issue #123)
- **Relacionadas:** [0003](0003-esquema-por-scripts-sql-idempotentes.md) (esquema por scripts), [0023](0023-edicao-em-massa-no-modo-edicao-de-transacoes.md) (desfazer guardado na API)

## Contexto

Muitas transações chegam do banco com descrições que não dizem o que foram ("PIX ENVIADO 12345", "Pagamento recebido"). Já dava para trocar o nome no formulário: a original fica em `Transacao.DescricaoOriginal`, e a sincronização não sobrescreve nome editado.

Faltavam duas coisas, pedidas na #123:
1. ver o nome do banco na lista, embaixo e menor;
2. o nome novo valer para as transações iguais que já existem e para as próximas.

A regra de descrição precisa de três coisas:
- **guardar**, com texto e tipo de correspondência iguais aos das regras de categoria;
- **aplicar na sincronização**;
- **saber quais transações têm "nome automático"**, para não mexer num nome escolhido à mão.

## Alternativas consideradas

- **Estender `RegraCategorizacao`** com `NovaDescricao`, deixando a categoria opcional.
  - Prós: uma tela e um motor de regras só.
  - Contras: `CategoriaId` é `NOT NULL`, e o SQLite não deixa mudar isso sem recriar a tabela (bloco de transição). A prioridade das regras de categoria também não se encaixa bem no nome.
- **Tabela nova `RegraDescricao`** (texto, tipo de correspondência, nova descrição, ativa).
  - Prós: `CREATE TABLE IF NOT EXISTS` no fim do `01-estrutura.sql`, sem transição.
  - Contras: a tela de regras ganha uma segunda tabela.
- **Para marcar o nome automático, só comparar `Descricao == DescricaoOriginal`.**
  - Prós: nenhuma coluna nova.
  - Contras: não distingue "nome dado por regra" de "nome editado à mão", nem guarda "quero o nome do banco mesmo com regra".

## Decisão

**Banco:**
- tabela `RegraDescricao`;
- em `Transacao`, `RegraDescricaoId` (a regra que deu o nome) e `DescricaoManual` (nome escolhido à mão; as regras não mexem mais).

**Nome automático:** `!DescricaoManual && DescricaoOriginal != null && (RegraDescricaoId != null || Descricao == DescricaoOriginal)`.
- Descrições editadas **antes** desta mudança têm nome diferente do banco e nenhuma regra, então ficam como estão, sem migração de dados.
- Lançamentos manuais não têm `DescricaoOriginal` e nunca entram.

**Qual regra vale** (`RegraDescricaoService`):
1. a mais específica: "é igual a", depois "começa com", depois "contém";
2. com o mesmo tipo, o texto mais longo;
3. com o mesmo texto, a mais antiga.

A comparação é com a descrição do banco, sem acento e sem diferença de maiúsculas.

**Onde as regras valem:**
- **Sincronização:** a transação de nome automático recebe o nome do banco e passa pelas regras.
- **Renomear com "também nas outras"** (`PUT /api/transacoes/{id}/descricao`):
  - cria a regra e reaplica todas, numa transação de banco;
  - recusa quando a regra não vale para a própria transação: o texto não bate, ou existe uma regra mais específica, que a mensagem aponta;
  - o "Desfazer" fica na memória da API por 10 minutos (como na ADR 0023) e apaga a regra criada.
- **Em massa** (`PATCH /api/transacoes/descricao/em-lote`): o mesmo nome nas selecionadas.
  - Só nelas: nome escolhido à mão.
  - Ou com uma regra "é igual a" por descrição do banco, no máximo 50 de uma vez. A regra que já existe com aquele texto passa a usar o nome novo, e o "Desfazer" volta o nome que ela tinha.
- **Criar pela tela de regras** (`POST /api/regras-descricao`, com prévia em `GET /api/regras-descricao/previa`): renomeia de uma vez as transações de nome automático que batem.
- **Editar, desligar ou excluir uma regra:** reaplica nas transações de nome automático. Ao excluir, as que tinham o nome dela voltam ao do banco, ou ao de outra regra.

**Outras consequências da decisão:**
- **Formulário e "Só nesta":** marcam o nome como escolhido à mão.
- **"Voltar à descrição do banco":** também marca à mão, para nenhuma regra renomear de novo.
- **Busca:** acha pelos dois nomes.
- **Categoria e etiquetas** (#111): continuam vindo da descrição do banco.

## Consequências

- **Positivas:**
  - nome claro em todo lugar que mostra a descrição (Painel, faturas, contas a pagar, relatórios);
  - o nome do banco sempre visível e pesquisável (Nielsen 6);
  - prévia com quantas transações mudam (Nielsen 5) e "Desfazer" (Nielsen 3);
  - transação renomeada sai do filtro "pouco informativa" (#9), que olha o nome mostrado.
- **Negativas:**
  - Duas listas de regras na mesma tela (categoria e descrição). A ordem de cada uma tem critério próprio: prioridade na de categoria, especificidade na de descrição.
  - Reaplicar percorre as transações de nome automático em memória (milhares, num computador só): aceitável aqui, mas não é o que se faria com milhões.
  - O "Desfazer" se perde se a API reiniciar nesses 10 minutos.
- **Na prática:**
  - Não grave `Transacao.Descricao` direto em código novo de sincronização ou importação: deixe o `RegraDescricaoService.Aplicar` decidir.
  - Ao trocar o nome por uma ação do usuário, marque `DescricaoManual`.
