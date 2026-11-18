# 0069. OCR do holerite escaneado ou foto: o Tesseract como programa à parte, opcional

- **Status:** Proposta (a escolha e o tamanho da instalação esperam o usuário)
- **Data:** 2026-11-14
- **Relacionadas:** [0068](0068-holerite-em-pdf-lido-no-go-com-ledongthuc-pdf.md) (o leitor do holerite em PDF), [0064](0064-modulos-compilados-no-executavel-e-ligados-no-banco.md) (módulos), [0029](0029-instalador-msi-com-wix-gerado-pela-ci.md) (`.msi`), [0034](0034-sem-sri-no-build-do-front-end.md) (CSP), [0049](0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md) (Go sem cgo)

## Contexto

A #228 lê o holerite em PDF **com texto** (ADR 0068). O leitor recebe as palavras com a posição na página e faz o resto: as linhas, as colunas, as rubricas, o dicionário, a prova real e a prévia. A #229 pede o holerite **sem texto**: o PDF escaneado, a foto do papel no celular, a imagem que o RH manda (JPG ou PNG).

**Falta só a primeira etapa:** um OCR que devolva as palavras **com a posição** e, de preferência, com a **confiança** de cada uma. A confiança serve para destacar as "linhas com leitura duvidosa" que a issue pede.

Restrições:
- **A imagem tem CPF e salário:** nenhum OCR de fora, nenhuma saída de rede.
- **Windows e Linux, executável Go sem cgo** (ADR 0049).
- **CSP restritiva:** hoje é `script-src 'self'`, sem `'wasm-unsafe-eval'` (ADR 0034).
- **Instalação:** o `.msi` (ADR 0029) e os scripts do Linux. O que entra neles pesa para todo mundo, mesmo para quem nunca vai usar o OCR.
- **O PDF escaneado** costuma ter uma imagem por página, quase sempre em JPEG (*inferido*: alguns scanners usam CCITT ou JBIG2). Antes do OCR, o site tem de tirar essa imagem do PDF.

## Alternativas consideradas

- **Tesseract** ([Apache-2.0](https://github.com/tesseract-ocr/tesseract)) **como programa à parte, chamado pela API** (`os/exec`, saída TSV com a caixa e a confiança de cada palavra). *Recomendada, julgamento a confirmar.*
  - Prós:
    - É o OCR livre mais usado, com modelo treinado em português.
    - O modelo rápido (`tessdata_fast/por.traineddata`) tem cerca de 2 MB (*inferido*, pelo espelho público; conferir no repositório oficial).
    - Dá a confiança de cada palavra.
    - Sem cgo, e o mesmo caminho no Windows e no Linux.
    - O Go só lê o TSV e reaproveita o `leitor.Agrupar`/`Interpretar`: cada palavra entra como uma "letra" larga.
  - Contras:
    - **Um processo novo**, que é uma fronteira no modelo de ameaças. O Tesseract lê a imagem da pessoa, de uma pasta temporária e com tempo limite.
    - **Precisa estar instalado**, no `.msi` ou à parte. O tamanho no Windows não foi medido: *inferido* na casa das dezenas de MB com as bibliotecas.
    - No Linux, a versão vem do pacote da distribuição (`tesseract-ocr`, `tesseract-ocr-por`).
    - **Foto torta ou com sombra** pede preparo da imagem (endireitar, contraste). O Tesseract faz parte disso. A perspectiva da foto de celular, não.
- **tesseract.js** (Apache-2.0) **no navegador**, com WebAssembly.
  - Prós:
    - Nada a instalar além do site, igual no Windows e no Linux.
    - A imagem nem chega à API: o navegador manda só as palavras com a posição.
  - Contras:
    - **A CSP teria de aceitar `'wasm-unsafe-eval'`**, e um *worker* (às vezes `blob:`, *inferido* pelos relatos públicos). Isso afrouxa a CSP do site inteiro, uma decisão de segurança.
    - O build cresce com o núcleo em WebAssembly e o modelo, mesmo carregados só ao abrir o leitor.
    - O PDF escaneado também teria de ser aberto no navegador.
- **`gosseract`** (o Tesseract dentro do Go): usa cgo, o que quebra a compilação cruzada da ADR 0049. Rejeitada.
- **OCR do próprio Windows** ([`Windows.Media.Ocr`](https://learn.microsoft.com/uwp/api/windows.media.ocr)).
  - Prós: já vem no Windows 10 e 11 e dá a caixa de cada palavra (`OcrWord.BoundingRect`).
  - Contras:
    - Depende do pacote de idioma português instalado.
    - Não dá a confiança por palavra (*inferido*).
    - Chamar o WinRT do Go sem cgo é trabalhoso.
    - **Não existe no Linux:** lá, o Tesseract seria necessário de qualquer jeito. Ficariam dois caminhos para manter.
- **OCR na nuvem** (Google, AWS, Azure, IA): manda a imagem do holerite para fora. Rejeitada.
- **Modelo de IA local** (PaddleOCR, ONNX): pesado, e pede cgo ou um runtime nativo. Rejeitada por agora.

## Decisão (proposta)

**Usar o Tesseract como programa à parte, chamado pela API, e opcional.** A ADR fica Proposta até o usuário decidir três pontos:

1. **De onde vem o Tesseract.** Opções:
   - **(a)** a pessoa instala à parte (`winget install UB-Mannheim.TesseractOCR` no Windows, `apt install tesseract-ocr tesseract-ocr-por` no Linux), e o site o acha na pasta padrão ou no caminho configurado (nunca pelo `PATH`, veja "Onde o site acha o Tesseract");
   - **(b)** o `.msi` traz o Tesseract junto.
   *Julgamento:* começar pela (a), sem crescer a instalação de quem não usa, e trazer no `.msi` só se o uso pedir. Antes de escolher a (b), medir o tamanho real no `.msi`.
2. **Ligado onde.** *Julgamento:* uma opção do módulo Trabalho, em Configurações › Módulos, que só aparece ligável com o Tesseract encontrado. Desligada, o leitor recusa o PDF sem texto como hoje.
3. **O modelo:** `tessdata_fast` (menor e mais rápido) ou `tessdata_best` (mais preciso, maior). *Julgamento:* medir os dois nos holerites fictícios antes.

**Como ficaria o caminho**, *julgamento a detalhar na implementação*:
1. **A entrada:**
   - o PDF sem texto (`leitor.ErrSemTexto`), de que o site tira a imagem de cada página (JPEG direto; os outros formatos, se der);
   - ou um JPG/PNG enviado direto.
2. **Os limites** (revisão do #707, 16/11/2026: os 700 KB do PDF não servem para foto, que sai do celular com 2 a 5 MB):
   - **O front-end reduz a foto antes de enviar:** o lado maior vai a no máximo 3000 pixels, em JPEG, até caber em
     700 KB (no `canvas`, sem biblioteca de fora e sem mudar a CSP). Assim o pedido segue no limite de 1 MB do site.
     3000 pixels no lado maior de uma folha A4 dão uns 250 dpi, perto dos 300 que o Tesseract recomenda (*inferido*:
     medir nos holerites fictícios fotografados).
   - **A API confere de novo, pelo conteúdo:** só JPEG e PNG (pelo começo do arquivo), até 700 KB, e **até 4000
     pixels de lado e 16 megapixels**, lidos do cabeçalho (`image.DecodeConfig`) antes de abrir a imagem. A imagem
     tirada do PDF escaneado passa pela mesma conferência. Os números são *inferidos*: uma página A4 escaneada a 300 dpi
     tem 2480 × 3508 pixels (8,7 megapixels).
   - **O PDF escaneado** continua com os 700 KB e as 5 páginas, e passa pelo mesmo leitor, com os limites da revisão do
     #706 (a árvore de páginas, o tamanho aberto, um PDF por vez e o prazo; ADR 0068). Tirar a imagem da página também
     confere o tamanho aberto antes.
3. **O OCR:** a imagem vai para uma pasta temporária com permissão só do serviço. O Tesseract roda com saída TSV, e a pasta é apagada no fim, mesmo com erro. A imagem não é guardada nem vai para o log, como o PDF.
   - **Limites do processo** (revisão do #707: imagem malformada também trava ou estoura a memória do Leptonica, como o
     PDF da #706): prazo por página (30 segundos, *inferido*), e no fim do prazo o processo é morto; **limite de memória
     do processo**, pelo sistema: no Windows, um *Job Object* com limite de memória e "matar ao fechar"
     (`golang.org/x/sys/windows`, que o site já usa); no Linux, o `prlimit` do util-linux (`--as` e `--cpu`), presente
     nas distribuições (*inferido*: conferir no Debian, no Ubuntu e no Fedora). Número inicial: 512 MB (*inferido*,
     medir).
   - **Um OCR por vez,** como a leitura do PDF.
   - **A saída também tem teto:** o TSV é lido até um limite (2 MB, *inferido*), e as palavras, até o limite de letras
     do leitor.

**Onde o site acha o Tesseract** (revisão do #707): **nunca pelo `PATH`.** O site roda como serviço do sistema, e
procurar pelo `PATH` deixaria outro programa com o mesmo nome rodar com a conta do serviço. O caminho é:
- **o padrão de cada sistema:** a pasta do instalador da UB-Mannheim no Windows (`C:\Program Files\Tesseract-OCR\tesseract.exe`)
  e `/usr/bin/tesseract` no Linux;
- **ou um caminho configurado** em Configurações, absoluto.
- **Conferido antes de ligar:** o arquivo existe, `tesseract --version` responde em poucos segundos e diz a versão 5 ou
  mais nova, e o português (`por`) está na lista de idiomas. *Julgamento:* avisar quando a pasta do programa aceita
  escrita da conta do serviço (no Windows, dentro de `Program Files` só o administrador escreve).
- O Go chama o caminho absoluto (`exec.Command` com o caminho inteiro não consulta o `PATH`).
4. **Do OCR ao leitor:** cada palavra vira uma "letra" do `leitor.Agrupar`, com a confiança. Daí em diante, o caminho da #228 sem mudança: o dicionário, as escolhas, a prova real e a prévia.
5. **Leitura duvidosa:** a linha com uma palavra de confiança baixa vem marcada, em texto, na prévia ("leitura duvidosa: confira com o papel").

## Consequências

- **Positivas:**
  - Quem recebe o holerite em papel ou escaneado também confere em vez de digitar.
  - O caminho da #228 é reaproveitado por inteiro.
  - Nada sai do computador, e quem não usa não paga o tamanho, na opção (a).
- **Negativas:**
  - **Um programa de fora** para instalar e manter atualizado, e uma fronteira nova no modelo de ameaças (o processo do OCR lendo um arquivo da pessoa).
  - **Limitar a memória do processo é diferente em cada sistema** (o *Job Object* no Windows, o `prlimit` no Linux):
    dois caminhos para manter e testar.
  - **A foto reduzida no navegador** perde detalhe: se 3000 pixels não bastarem para o OCR, o limite sobe com o pedido
    maior (e o limite de 1 MB do site muda junto, outra decisão).
  - **A foto de celular em ângulo pode não ser lida:** sem corrigir a perspectiva, o OCR erra. A prova real e a "leitura duvidosa" mostram o erro, mas não o corrigem.
  - **Os testes de OCR precisam do Tesseract na CI** (`apt install` no Ubuntu e `choco` ou `winget` no Windows). Localmente, sem ele, o teste é pulado e diz isso.
  - **As imagens de teste** têm de ser geradas na hora, a partir dos holerites fictícios do `leitor/ficticios`: inclinadas, com sombra, em baixa resolução. Nunca imagem real.
- **Na prática (quando aceita):**
  - atualizar a ADR 0049 (o programa de fora), o `docs/DADOS-PESSOAIS.md` (a imagem), o Manual e o CHANGELOG;
  - no `docs/MODELO-DE-AMEACAS.md`, as linhas do processo novo: **elevação de privilégio** (o caminho fixo ou
    configurado, conferido, nunca o `PATH`), **negação de serviço** (o prazo, o limite de memória do processo, a imagem
    limitada em bytes e em pixels antes de abrir, um OCR por vez, o teto da saída) e **vazamento** (a pasta temporária
    só do serviço, apagada no fim, nada no log);
  - na opção (b), também o `instalador/` e o workflow do `.msi`, com o tamanho antes e depois no PR ([requisitos não funcionais](../REQUISITOS-NAO-FUNCIONAIS.md)).
