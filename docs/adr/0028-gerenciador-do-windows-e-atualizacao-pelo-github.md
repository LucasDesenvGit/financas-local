# 0028. Gerenciador do Windows, com atualização pelo GitHub Releases

- **Status:** Substituída por [0052](0052-gerenciador-em-go-com-bandeja-e-pagina-local.md): o gerenciador em C# saiu na #86 (fica na tag `v-ultimo-dotnet`). Antes, em parte substituída pela [0040](0040-atualizar-pelo-develop-so-na-instalacao-de-desenvolvimento.md) (atualizar pelo develop na instalação de desenvolvimento)
- **Data:** 2026-10-17 (issue #119); encurtada em 2026-11-15 (#712)
- **Relacionadas:** [0033](0033-gerenciador-no-linux-com-avalonia.md) (gerenciador no Linux), [0029](0029-instalador-msi-com-wix-gerado-pela-ci.md) (instalador .msi)

## Contexto

Depois de instalado, o site é um serviço do Windows sem janela. Faltava ao usuário, sem terminal: saber se o site está no ar, ligá-lo ou desligá-lo, atualizar e tirar uma cópia do banco, que fica numa pasta que só SYSTEM e Administradores leem (issue #119).

## Alternativas consideradas

- **Botão "Atualizar" no próprio site:** a API roda como LocalSystem; deixar o site baixar e executar instaladores transforma qualquer falha web numa execução como SYSTEM, sem UAC visível.
- **Atualização automática e silenciosa:** o mesmo problema de privilégio, e uma versão quebrada chega sem a pessoa saber.
- **Squirrel, ClickOnce ou winget:** não lidam com serviço do Windows (os dois primeiros) ou exigem um repositório da Microsoft (o winget).
- **App pequeno do Windows, instalado pelo .msi.** **Escolhida.**

## Decisão

- **Um app WinForms** (`Financas.Gerenciador`), que roda como o usuário e pede o **UAC só ao ligar, desligar ou copiar o banco** (reabrindo a si mesmo com `runas` e argumentos fixos).
- **Na janela:** primeiro a situação do site, com a versão; depois a versão nova, se houver, com "O que mudou"; o resto em "Mais opções" (a cópia do banco com `VACUUM INTO`, iniciar com o Windows, procurar versão nova ao abrir).
- **Atualização:** consulta o último Release do GitHub, baixa o `.msi` e o `SHA256SUMS.txt`, e só abre o instalador se o hash bater. Procurar versão nova **só avisa**: instalar é sempre um clique da pessoa.

## Consequências

- **Positivas:** a situação sempre visível, linguagem sem "serviço" ou "msiexec", e a elevação sempre num UAC ligado a uma ação que a pessoa pediu.
- **Negativas:**
  - **saída de rede nova**, só no gerenciador: `api.github.com`, `github.com` e o domínio de download do GitHub;
  - **sem assinatura de código**, o SHA256 prova que o arquivo é o publicado no Release, mas não quem o publicou;
  - **só Windows** (o Linux ganhou o dele na [0033](0033-gerenciador-no-linux-com-avalonia.md)).
- **O que continuou no gerenciador em Go ([0052](0052-gerenciador-em-go-com-bandeja-e-pagina-local.md)):** o UAC só nas ações que precisam, a atualização pelo GitHub Releases conferida pelo SHA256, o "só avisa" e as mesmas saídas de rede. O desenho da janela, a instalação pelos scripts e o núcleo comum com o Linux, que esta ADR detalhava, ficaram na tag `v-ultimo-dotnet`.
