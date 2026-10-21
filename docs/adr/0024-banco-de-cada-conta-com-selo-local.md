# 0024. Banco de cada conta e cartão, com selo desenhado no próprio site

- **Status:** Aceita
- **Data:** 2026-10-17
- **Relacionadas:** [0016](0016-icones-do-lucide-empacotados-no-build.md) (ícones empacotados no build), [0018](0018-estado-da-tela-na-url-e-detalhe-por-navegacao.md) (`?faturas=` na URL), [0021](0021-componentes-padrao-para-graficos-e-telas-densas.md) (detalhe sob demanda)

## Contexto

Na tela de Contas e cartões não dava para reconhecer de relance qual conta ou cartão era de qual banco (Nubank, Itaú, Mercado Pago…). Havia só o nome da instituição em texto pequeno, quando a Pluggy informava. Pelo Meu Pluggy, essa informação nem sempre é o banco de verdade (*inferido*). O usuário pediu para:
- indicar o banco de cada conta e a conta de cada cartão;
- ver o logo de cada banco.

Restrições:
- a CSP não deixa carregar imagem de fora (ADR 0016);
- tudo roda local;
- sair para um domínio novo exige o padrão das saídas de rede (README › Segurança).

## Alternativas consideradas

- **Logos oficiais baixados da Pluggy** (`imageUrl` dos conectores, em `cdn.pluggy.ai`), guardados no banco e servidos pela API.
  - Prós: logo oficial de qualquer banco que a Pluggy atende.
  - Contras:
    - domínio de saída novo, e o pedido de cada logo diz à CDN quais bancos a pessoa usa;
    - depende das credenciais da Pluggy;
    - servir SVG de terceiros pela própria origem pede cuidado extra (SVG pode conter script);
    - pelo Meu Pluggy, o conector é o próprio Meu Pluggy, não o banco.
- **Arquivos de logo oficiais no repositório** (kits de imprensa ou Wikimedia).
  - Prós: visual fiel.
  - Contras: arquivos de marca de terceiros versionados no projeto, com licença de uso incerta.
- **Simple Icons (licença CC0) onde houver, e selo próprio (cor da marca + sigla) nos demais**, tudo no build.
  - Prós: nada sai da máquina, licença livre e a CSP continua igual.
  - Contras:
    - o Simple Icons não tem Itaú, Bradesco, Santander, Banco do Brasil, Caixa, Inter, C6, BTG, XP;
    - para esses, o selo é uma aproximação, não o logotipo.
- **Só o nome do banco em texto.**
  - Prós: nenhuma decisão de marca.
  - Contras: não resolve o pedido (reconhecer de relance).

## Decisão

Cada conta tem um **banco**, que vale nesta ordem:
1. o **escolhido** pelo usuário (`Conta.Banco`; "Outro (sem banco)" desliga);
2. no cartão sem escolha, o banco da **conta a que ele pertence** (`Conta.ContaVinculadaId`, escolhida no cartão);
3. o **detectado** pelo nome da instituição informado pela Pluggy e pelo nome da conta, por palavras inteiras sem acento ("Nubank Ultravioleta" → Nubank).

O catálogo e a detecção ficam no `BancoService`, no back-end. A API devolve o código que vale (`ContaDto.Banco`) e se ele foi escolhido.

O **selo** é desenhado no front-end (`compartilhado/bancos.ts` e `app-logo-banco`), sem baixar nada:
- **Com desenho:** o da marca, do Simple Icons (CC0), copiado para o código (sem dependência npm nova) para Nubank, Mercado Pago, PicPay, PagBank, Nomad, Wise, Revolut e PayPal.
- **Selo próprio:** cor da marca + sigla para os demais.
- **Banco não identificado:** ícone neutro.
- **Leitor de tela:** o selo é decorativo quando o nome do banco vem escrito ao lado (WCAG 1.1.1).
- **Cor do cartão:** o cartão desenhado usa a cor do banco.

Na mesma mudança, o cartão passa a mostrar a **fatura do mês** (o que entrou no mês, a mesma conta do Painel, que foi para o `ContaService`) separada da **dívida total** (com parcelas futuras). O **histórico de faturas** sai do modal e fica recolhido no próprio cartão; `?faturas=ID` abre o histórico daquele cartão (ADR 0018).

## Consequências

- **Positivas:**
  - contas e cartões reconhecíveis de relance;
  - o cartão "com a cara" do banco;
  - nenhum domínio novo nem arquivo de logo oficial no repositório;
  - fatura do mês e dívida total com o mesmo significado no Painel e em Contas (Nielsen 4).
- **Negativas:**
  - para os bancos sem desenho no Simple Icons (Itaú, Bradesco, Santander, BB, Caixa, Inter, C6…), o selo é uma aproximação (cor + sigla), não o logotipo oficial;
  - **banco novo entra em dois lugares:** `BancoService.Catalogo` (código, nome, termos) e `compartilhado/bancos.ts` (selo). Sem os dois, o banco aparece sem selo (ícone neutro);
  - a detecção por palavras pode errar (ex.: conta chamada "Next" sem ser do banco Next). Por isso o banco é sempre editável, e "Outro" desliga;
  - `ContaVinculadaId` não tem FK no banco (coluna nova num SQLite já existente): a API limpa a referência ao excluir a conta;
  - as marcas continuam sendo dos donos; aqui servem só para identificar a conta na tela do próprio usuário.
- **Na prática:**
  - **Banco novo:** acrescentar no `BancoService` e em `bancos.ts`; o desenho, quando houver no Simple Icons, é o `path` do SVG dele.
  - **Selo em outra tela:** use `<app-logo-banco [codigo]="...">`, decorativo quando o nome vem escrito ao lado e com `rotulo` quando aparece sozinho.
  - **Fatura do mês:** é só o `ContaService.FaturasDoMesAsync`, para o Painel e Contas não divergirem.

## Atualização de 2026-10-20: selo das cobranças (issue #265)

- **O que mudou:** o mesmo selo local identifica também o **serviço de cada cobrança** em Contas a pagar (aba Contas a
  pagar, Assinaturas e recorrentes e Sugestões do extrato): YouTube, Netflix, Spotify, iFood, TIM, Claro, Esfera…
  - Lista em `compartilhado/servicos.ts`, gerada a partir do pacote `simple-icons` 16.35.0 (CC0): o desenho quando o
    Simple Icons tem a marca (46 dos 122), senão um selo próprio com a cor da marca e uma sigla, como os bancos sem desenho.
    A maioria das marcas brasileiras (TIM, Claro, Oi, 99, Rappi, Magalu, Sem Parar, Sabesp, Enel, Unimed…) não está no
    Simple Icons e fica com a sigla; as cores são aproximadas, e o selo não é o logotipo oficial. O Petlove tem um desenho
    próprio (uma pata), também não oficial (#269). O Pix fica por último (marca específica ganha) e o Mercado Pago depois do
    Mercado Livre ("MP*MERCADOLIVRE" é do Mercado Livre).
  - O serviço é reconhecido no front-end pelo texto da cobrança (e depois pelo nome), em ordem do mais específico para o
    mais geral; termo de até 4 letras só casa com a palavra inteira ("TIM" não casa com "TIMBRE").
  - Sem serviço conhecido: o **ícone da categoria** (Lucide, `IconesCategoria` em `icones.ts`) num quadrado neutro;
    categoria criada pela pessoa ou sem categoria: um recibo.
  - Componente **`<app-selo-servico>`**, sobre o `<app-logo-banco>` (que ganhou `visualPronto` e `iconeGenerico`).
    Decorativo: o nome vem escrito ao lado. É genérico, para qualquer lista de lançamentos: procura o serviço no texto
    do banco (`texto`), no estabelecimento e no nome, nessa ordem, e cai no ícone da categoria (as de receita e
    transferência também têm ícone). Usado em Transações (cada linha), Painel (Próximos vencimentos e Últimas
    transações) e Contas a pagar (lista, Assinaturas e recorrentes e Sugestões do extrato).
- **Por que no front-end, sem mudar a API:** é só a cara da cobrança; o texto já vem na lista. Sem rede: nada é
  baixado, a CSP não muda.
- **Custo aceito:** a lista de serviços é mantida à mão; serviço novo (ou que mudou de nome no extrato) fica com o ícone
  da categoria até entrar em `servicos.ts`. Um termo pode casar com um texto que não é do serviço (ex.: "VIVO" num nome
  próprio); o efeito é só o selo errado, e o nome continua escrito ao lado.
- **Na prática:** tela nova com lançamentos ou cobranças usa `<app-selo-servico>` com os textos que tiver (exemplos no
  comentário do componente). Serviço novo entra em `servicos.ts` (o `path` do SVG do Simple Icons, quando houver, ou cor e sigla),
  com o termo do extrato; o teste `testes/regressao/selo-cobranca.spec.ts` confere que todo serviço tem desenho ou sigla.
