# 0068. Holerite em PDF lido no Go, com a `ledongthuc/pdf`, sem guardar o arquivo

- **Status:** Aceita
- **Data:** 2026-11-14
- **Relacionadas:** [0049](0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md) (bibliotecas do Go), [0064](0064-modulos-compilados-no-executavel-e-ligados-no-banco.md) (módulos), [0065](0065-trabalho-emprego-salario-com-vigencia-e-tabelas-oficiais.md) (Trabalho), [0026](0026-manual-do-site-como-dados-no-front-end.md) (dado no código), [0027](0027-conferencia-de-investimentos-com-previa-e-desfazer.md) (prévia e desfazer), [0030](0030-regras-de-descricao-em-tabela-propria.md) (texto normalizado e a escolha lembrada)

Os passos da leitura em detalhe e o relato da revisão do #706 (os PDFs que travaram a leitura) ficaram na [versão anterior desta ADR](https://github.com/LucasDesenvGit/financas-local/blob/2790797b27114f4f7561300c2084101cad9a57cd/docs/adr/0068-holerite-em-pdf-lido-no-go-com-ledongthuc-pdf.md).

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

**O PDF é lido na API, em Go, pela `ledongthuc/pdf`, e não é guardado.** O leitor (`internal/modulos/trabalho/leitor`, sem HTTP) não tem um modelo por empresa: ele se apoia na anatomia comum.

1. **As letras com a posição viram linhas e células** (linha: a mesma altura, com folga; célula: separada por um vão).
2. **A tabela** começa na linha com os títulos "Vencimentos" (ou "Proventos", "Créditos") e "Descontos" (ou "Débitos"), cuja posição diz a coluna, e acaba nos totais, no líquido ou nas bases. Cada linha é uma rubrica; é provento ou desconto pela coluna mais perto do valor. Com duas vias na página, vale a primeira.
3. **O rodapé** dá os totais, o líquido, o salário base e as bases do INSS, do FGTS e do IRRF, pelo rótulo.
4. **A prova real** (soma de cada coluna contra o total impresso; proventos − descontos contra o líquido) diz na tela qual total não bate e por quanto.

O `holeritepdf` classifica as rubricas: o **dicionário fica no código** (`dicionario.go`, expressões normalizadas como na ADR 0030), e **o que a pessoa escolhe fica no banco** (`HoleriteRubricaEscolha`, por emprego, pelo código e pela descrição normalizada). No mês seguinte a rubrica já vem reconhecida.

**Como o arquivo chega:** JSON com o PDF em base64, para caber no limite de 1 MB do site (o PDF vai até 700 KB e 5 páginas); só PDF, conferido pelo começo do arquivo (`%PDF-`); a biblioteca só extrai texto (script, formulário e anexo não rodam); a senha só abre o arquivo, não é guardada nem vai para o log; o erro e o log não levam texto do PDF. Guardar o PDF como anexo fica para outra decisão.

**Limites da leitura:** a biblioteca não se protege de um PDF malformado, então o `leitor/conteudo.go` percorre as páginas com limite, confere o tamanho aberto antes de ler, tem teto de letras por página e lê um PDF por vez, com prazo. Os números e o que fica de fora estão no [modelo de ameaças](../MODELO-DE-AMEACAS.md) (negação de serviço). Para pôr esses limites, a extração do texto é uma cópia da conta do `Content()` da biblioteca.

**O que é gravado:** nada sem "Confirmar", depois de uma prévia com cada linha, o tipo, as provas e os avisos. O holerite lido entra fechado, com `Origem` = 2 e o cálculo montado das linhas confirmadas, e por isso o 13º, as férias, o FGTS e a rescisão o usam sem mudança ([domínio](../arquitetura/dominio.md)). No mês fechado, o site pede para reabrir antes. "Desfazer" por 10 minutos, na memória do processo, como a ADR 0027.

## Consequências

- **Positivas:**
  - A pessoa confere em vez de digitar, e o histórico (base do 13º, das férias e da rescisão) fica sem buracos.
  - Nada sai do computador, e nenhuma dependência com C entra no executável.
  - Rubrica nova se ensina uma vez por emprego. Nome comum novo entra no dicionário com um teste.
- **Negativas:**
  - **Não reconhece 100% dos holerites.** Um layout fora da anatomia comum (uma coluna só de "Valor" com o sinal) não é lido: a prova real mostra o erro, ou a tela diz "não achei a tabela".
  - Depende de uma biblioteca pequena. Se ela parar, o leitor fica na última versão que funciona; trocar mexe só no `leitor/pdf.go` e no `leitor/conteudo.go`.
  - Uma versão nova da biblioteca que mude a conta da extração não muda o leitor sozinha; o teste `TestMesmaContaDaBiblioteca` avisa quando as duas se separam.
  - Um laço da biblioteca ao abrir o PDF escapa do prazo e prende uma goroutine até o site reiniciar; ler num processo à parte resolveria, mas é outra fronteira (*inferido*; o OCR, ADR 0069, vai ter um).
  - Holerite escaneado ou foto não tem texto: fica para o OCR (#229).
  - O base64 aumenta o pedido em um terço, e o limite de 700 KB é menor que o de 1 MB do site.
  - O "Desfazer" some se o serviço reiniciar nos 10 minutos. Aí, o caminho é "Reabrir" (com registro na auditoria) e "Excluir".
- **Na prática:**
  - **Nunca holerite real no repositório.** Os testes usam os PDFs fictícios do `leitor/ficticios`, gerados na hora; o com senha é um arquivo pronto (`ficticios/testdata`), porque o código não cifra nada (o CodeQL não aceita RC4). Layout novo entra lá, com empresa, CNPJ e valores inventados.
  - **A rubrica que o site não conhece vem sem tipo** na prévia, e o "Confirmar" espera a pessoa escolher ou deixar a linha de fora.
  - **Nome comum novo:** a expressão normalizada vai na regra do tipo em `holeritepdf/dicionario.go`, com teste.
  - **A `ledongthuc/pdf` está na tabela de bibliotecas da ADR 0049.** Trocar de biblioteca pede uma ADR nova.
