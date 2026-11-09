# 0062. QR code do acesso pelo celular desenhado no site, com o qrcode-generator

- **Status:** Aceita
- **Data:** 2026-11-09
- **Relacionadas:** [0061](0061-acesso-pelo-celular-na-rede-local.md) (acesso pelo celular), [0016](0016-icones-do-lucide-empacotados-no-build.md) (ícones empacotados), [0004](0004-front-end-publicado-no-wwwroot-da-api.md) (CSP restritiva)

## Contexto

Em Configurações › Segurança › "Acesso pelo celular" (#328), o passo a passo pedia para digitar no celular dois endereços longos: o do certificado (`https://<ip>:5443/api/celular/autoridade`) e o do site. Digitar IP, porta e caminho no teclado do celular é lento e fácil de errar. O usuário pediu um QR code para abrir pelo celular (#677).

Gerar um QR code exige a codificação da norma (ISO/IEC 18004: modos, versões, correção de erro Reed-Solomon e máscaras). A CSP do site não permite nada de fora, nem `innerHTML` com SVG vindo de texto sem passar pelo sanitizador do Angular.

## Alternativas consideradas

- **Escrever o codificador no projeto:** sem dependência, mas são centenas de linhas de matemática de norma (Reed-Solomon, tabelas de versão e máscara) para manter, e um erro só aparece quando um celular não lê.
- **Gerar no back-end em Go (`rsc.io/qr` ou `skip2/go-qrcode`) e servir uma imagem:** dependência nova no módulo do site, rota nova e uma imagem que muda quando muda o IP. O endereço já está no front-end.
- **`qrcode` (npm, o mais baixado):** traz dependências (`pngjs`, `dijkstrajs`, `yargs` no pacote), mais do que precisamos para desenhar uma matriz.
- **`qrcode-generator` (npm, Kazuhiko Arase, licença MIT):** sem nenhuma dependência, com tipos do TypeScript, e devolve a matriz (`isDark(linha, coluna)`). Versão 2.0.4, de agosto de 2025.

## Decisão

Usamos o `qrcode-generator` na versão **fixa** `2.0.4`, empacotado no build, só para calcular a matriz. O desenho é do site: a função pura `desenharQr` (`paginas/ajustes/acesso-celular.ts`) monta um `<path>` de SVG com um quadradinho por módulo escuro, e o `<app-qr-code>` (`paginas/ajustes/qr-code.component.ts`) mostra esse caminho no template, por `[attr.d]`, sem `innerHTML`. A CSP não muda.

- Correção de erro média (M: cerca de 15% do código pode estar borrado ou fora de foco) e a margem branca de 4 módulos que a norma pede.
- Sempre preto no branco, também no tema escuro: muitos leitores não leem o código invertido.
- O endereço continua escrito no passo a passo; o QR é um atalho, com `role="img"`, nome para o leitor de tela e legenda.

## Consequências

- **Positivas:**
  - o celular abre o certificado e o site pela câmera, sem digitar;
  - nada baixado de fora, e a CSP continua igual;
  - a biblioteca só entra no trecho da tela de Configurações, que já é carregada sob demanda.
- **Negativas:**
  - mais uma dependência de terceiros no front-end, com versão fixa: atualizações dependem de alguém subir a versão à mão;
  - o teste automático confere a estrutura do código (os três quadrados de canto e a margem), mas não lê o QR: quem confirma a leitura é a câmera de um celular de verdade (*inferido*: o código segue a norma porque a biblioteca é usada há anos, e não porque o projeto a valida).
- **Na prática:** outro QR code no site usa o `desenharQr` e o `<app-qr-code>`; se mais de uma tela precisar, os dois vão para `compartilhado/ui/`.
