# 0068. Holerite em PDF lido no Go, com a `ledongthuc/pdf`, sem guardar o arquivo

- **Status:** Proposta
- **Data:** 2026-11-14
- **Relacionadas:** [0049](0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md) (bibliotecas do Go), [0064](0064-modulos-compilados-no-executavel-e-ligados-no-banco.md) (módulos), [0065](0065-trabalho-emprego-salario-com-vigencia-e-tabelas-oficiais.md) (Trabalho), [0026](0026-manual-do-site-como-dados-no-front-end.md) (dado no código), [0027](0027-conferencia-de-investimentos-com-previa-e-desfazer.md) (prévia e desfazer), [0030](0030-regras-de-descricao-em-tabela-propria.md) (texto normalizado e a escolha lembrada)

## Contexto

No Trabalho (#209), o holerite do mês (#211) é digitado linha a linha, todo mês. A #228 pede para mandar o **PDF do holerite** e o site preencher o mês: proventos, descontos, bases e líquido. A pessoa só confere.

Restrições:
- **O holerite tem CPF, salário e empregador.** Nada pode sair do computador: nem serviço de fora, nem IA na nuvem.
- **Não há um modelo de holerite.** Cada sistema de folha desenha o seu, mas quase todos têm a mesma anatomia:
  - o cabeçalho, com o CNPJ e a competência;
  - a tabela Código · Descrição · Referência · Vencimentos · Descontos;
  - os totais e o líquido;
  - o rodapé com as bases (INSS, FGTS, IRRF).
- **PDF com senha é comum** (parte do CPF, a data de nascimento).
- **O épico Trabalho é em Go** (#231). O executável é compilado para Windows e Linux sem C (ADR 0049).
- **A API recebe no máximo 1 MB por pedido** (`servidor.LimiteDoCorpo`).

## Alternativas consideradas

- **Na API, em Go, com a [`github.com/ledongthuc/pdf`](https://github.com/ledongthuc/pdf)** (escolhida).
  - Prós:
    - Go puro, sem dependência nenhuma (o `go.mod` dela não tem `require`), licença BSD-3: é um fork mantido do `rsc.io/pdf`, do time do Go.
    - Dá o texto **letra a letra, com a posição** (`Page.Content().Text`: X, Y, largura, fonte). É o que a leitura por colunas precisa.
    - Abre PDF com senha. Conferido no código: RC4 de 40 a 128 bits (revisões 2 e 3) e AES-128 (V4). Conferido no teste: um PDF com RC4 de 40 bits abre com a senha certa, e a errada dá `ErrInvalidPassword`.
    - Os testes ficam no `go test` que já existe, com os PDFs gerados na hora.
  - Contras:
    - Biblioteca pequena, de uma pessoa, com versão sem tag (pseudo-versão).
    - Pode entrar em pânico com um PDF malformado: o leitor faz `recover`.
    - Não lê AES-256 (revisões 5 e 6): esse PDF dá "não abriu" (*inferido*: holerite com AES-256 é raro).
- **No navegador, com o pdf.js (`pdfjs-dist`, Apache-2.0).**
  - Prós: o PDF nem chega à API, e os testes ficam no Jest.
  - Contras:
    - O build cresce, e o *worker* do pdf.js precisa caber na CSP (`'self'`, ADR 0034). O tamanho não foi medido: *inferido* pelo tamanho público do pacote, na casa dos megabytes.
    - A regra (dicionário, escolhas, lido × calculado) ficaria dividida entre o front-end e a API, que guarda o holerite.
- **Outra biblioteca Go.**
  - `pdfcpu` (Apache-2.0): feita para manipular PDF (juntar, cortar, carimbar). Não dá o texto com a posição de cada letra.
  - `unipdf`: licença AGPL ou comercial.
  - `go-fitz` (MuPDF): usa cgo, o que quebra a compilação cruzada, e é AGPL.
- **IA na nuvem**: manda o holerite para fora, contra o modelo de segurança do README. Rejeitada.
- **Modelo de IA local**: pesado demais para instalar junto com o site. Rejeitada.
- **Dicionário de rubricas numa tabela do banco, editável na tela** × **no código**.
  - O banco deixa a pessoa mudar o dicionário, mas pede uma tela e uma migração para cada nome novo.
  - O código é como o Manual (ADR 0026): é dado, testado, revisado no PR e aumentado sem tela.
  - O que muda de pessoa para pessoa (a rubrica que só aquela empresa usa) vai para a tabela das escolhas.

## Decisão

**O PDF é lido na API, em Go, pela `ledongthuc/pdf`, e não é guardado.** O leitor (`internal/modulos/trabalho/leitor`, sem HTTP) não tem um modelo por empresa. Ele se apoia na anatomia comum:

1. **As letras com a posição viram linhas e células.**
   - Linha: a mesma altura, com 2 pontos de folga.
   - Célula: um vão de mais de uma letra e meia separa duas.
2. **A tabela.**
   - Começa na linha com "Vencimentos" (ou "Proventos", "Créditos") e "Descontos" (ou "Débitos"). A posição de cada título diz a coluna.
   - Cada linha é uma rubrica: o código, a descrição, a referência e o valor. É provento ou desconto pela coluna mais perto do valor.
   - Acaba nos totais, no líquido ou nas bases.
   - Com duas vias na mesma página, vale só a primeira.
3. **O rodapé** acha os rótulos conhecidos e o valor de cada um, na mesma célula, à direita ou embaixo: total de vencimentos, de descontos, líquido, salário base, base do INSS, do FGTS, FGTS do mês e base do IRRF.
4. **A prova real:**
   - a soma de cada coluna contra o total impresso;
   - proventos − descontos contra o líquido impresso.
   Quando não bate, a tela diz qual total e por quanto.

O `holeritepdf` classifica as rubricas:
- **O dicionário fica no código** (`dicionario.go`). Cada tipo tem as expressões já normalizadas (ADR 0030), e vale a primeira regra que casa, do lado em que o valor estava.
- **O que a pessoa escolhe fica no banco**, na tabela `HoleriteRubricaEscolha`, por emprego. A chave é o código e a descrição normalizada.
- No mês seguinte, a rubrica já vem reconhecida. A escolha que o dicionário já dá sozinho é esquecida.

**Como o arquivo chega e o que fica:**
- **JSON com o PDF em base64**, para caber no limite de 1 MB do site: o PDF vai até **700 KB e 5 páginas**.
- **Só PDF**, conferido pelo começo do arquivo (`%PDF-`) e não pela extensão.
- **A biblioteca só extrai texto:** script, formulário e anexo do PDF não rodam.
- **A senha** só abre o arquivo: não é guardada e não vai para o log.
- **O erro não leva texto do PDF**, e o log da falha não leva o conteúdo.
- **O PDF não é guardado.** Guardar como anexo fica para outra decisão.

**Limites da leitura (revisão do #706, 16/11/2026):** o PDF vem de quem usa o site, e a biblioteca não se protege de um
PDF malformado. Na revisão, um PDF de uns 200 bytes (a árvore de páginas com um `/Pages` filho dele mesmo) travou o
`r.Page` para sempre, e um de 143 KB (o conteúdo em FlateDecode) abriu para 39 MB e levou o processo a uns 12 GB de
memória. Por isso o `leitor/conteudo.go`:
- **percorre a árvore de páginas e a cadeia de `Parent` com limite** (8 níveis, 64 nós): ciclo vira "o PDF não abriu";
- **confere o tamanho aberto antes de ler:** 1 MB de conteúdo por página e 128 KB do `ToUnicode` de cada fonte, lidos
  com teto (o resto nem é descomprimido);
- **tira as letras com a mesma conta do `Content()` da biblioteca** (a posição de cada letra, conferida letra a letra
  num teste contra a própria biblioteca), mas com **teto de 20 mil letras por página**, conferido antes de traduzir: a
  tradução de um código pelo `ToUnicode` pode ser um texto enorme, e o teto usa o maior texto da tabela;
- **lê um PDF por vez, com prazo de 5 segundos**, conferido a cada operador do conteúdo.
- **O que fica de fora:** um laço da biblioteca que o prazo não alcança (ao abrir o PDF: a cadeia `/Prev` da tabela xref
  ou `/Extends` dos fluxos de objetos, em ciclo) não para: a leitura desiste de esperar no prazo, mas a goroutine fica
  presa, com a vaga, e as próximas leituras dizem "o leitor está lendo outro PDF" até o site reiniciar. Uma CPU ocupada,
  e não uma a cada tentativa. Ler num processo à parte, com limite de memória e de tempo do sistema, resolveria tudo,
  mas é outra fronteira (*inferido*; a #229 do OCR já vai ter um processo assim, ADR 0069).
- Um holerite de verdade fica muito abaixo de todos os limites (uns 20 KB de conteúdo e 3 mil letras por página).

**O que é gravado:**
- **Nada sem "Confirmar":** a prévia mostra cada linha, o tipo, o trecho do PDF, as provas e os avisos (CNPJ, mês, salário, lido × calculado).
- **O holerite lido é o do mês, fechado, com `Origem` = 2.** O cálculo guardado é montado com as linhas confirmadas. Assim, o 13º, as férias, o FGTS, a rescisão e o recebimento usam o lido sem mudança.
- **No mês com a estimativa aberta**, o lido toma o lugar dela e guarda os lançamentos. Reabrir volta para a estimativa.
- **No mês fechado**, o site pede para reabrir antes.
- **"Desfazer"** por 10 minutos, na memória do processo, como a ADR 0027.

## Consequências

- **Positivas:**
  - A pessoa confere em vez de digitar, e o histórico (base do 13º, das férias e da rescisão) fica sem buracos.
  - Nada sai do computador, e nenhuma dependência com C entra no executável.
  - Rubrica nova se ensina uma vez por emprego. Nome comum novo entra no dicionário com um teste.
- **Negativas:**
  - **Não reconhece 100% dos holerites.** Um layout fora da anatomia comum não é lido, como o de uma coluna só de "Valor" com o sinal. Nesse caso, a prova real mostra o erro, ou a tela diz "não achei a tabela".
  - Depende de uma biblioteca pequena. Se ela parar, o leitor fica na última versão que funciona, e trocar mexe só no `leitor/pdf.go` e no `leitor/conteudo.go`.
  - A extração do texto é uma cópia da conta da biblioteca (para pôr os limites): uma versão nova dela que mude a conta
    não muda o leitor sozinha. O teste `TestMesmaContaDaBiblioteca` avisa quando as duas se separam.
  - Holerite escaneado ou foto não tem texto: fica para o OCR (#229).
  - O base64 aumenta o pedido em um terço, e o limite de 700 KB é menor que o de 1 MB do site.
  - O "Desfazer" some se o serviço reiniciar nos 10 minutos. Aí, o caminho é "Reabrir" (com registro na auditoria) e "Excluir".
- **Na prática:**
  - **Nunca holerite real no repositório.** Os testes usam os PDFs fictícios do `leitor/ficticios`, gerados na hora (quatro estilos de sistemas de folha). O com senha é um arquivo pronto (`ficticios/testdata`), gerado uma vez da via clássica: o código não cifra nada (o CodeQL não aceita RC4, revisão do #706). Layout novo entra lá, com empresa, CNPJ e valores inventados.
  - **A rubrica que o site não conhece vem sem tipo** na prévia, e o "Confirmar" espera a pessoa escolher (ou deixar a linha de fora): o "Outro" que o site sugeria não vira escolha lembrada sem ela escolher (revisão do #706). O rodapé do PDF (salário base, bases do INSS e do FGTS, FGTS do mês) vai de R$ 0,00 a R$ 9.999.999,99, como as linhas.
  - **Nome comum novo:** a expressão normalizada vai na regra do tipo em `holeritepdf/dicionario.go`. O teste confere a normalização e o caso.
  - **A `ledongthuc/pdf` está na tabela de bibliotecas da ADR 0049.** Trocar de biblioteca pede uma ADR nova.
  - **O critério de aceite com um holerite real** (#228) é conferido pelo usuário, fora do repositório.
