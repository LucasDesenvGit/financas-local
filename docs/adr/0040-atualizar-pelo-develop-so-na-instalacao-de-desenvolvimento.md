# 0040. Atualizar pelo develop, pelo gerenciador, só na instalação de desenvolvimento

- **Status:** Aceita
- **Data:** 2026-10-20 (issue #154, parte 2)
- **Relacionadas:** substitui em parte a [0028](0028-gerenciador-do-windows-e-atualizacao-pelo-github.md) (o gerenciador só instala versão publicada; na instalação pelos scripts, não procura nem instala versões) e a [0033](0033-gerenciador-no-linux-com-avalonia.md) (o gerenciador do Linux não acessa a internet; a atualização só mostra o comando); ajusta a [0029](0029-instalador-msi-com-wix-gerado-pela-ci.md) (o gerenciador só consulta `releases/latest`); [0039](0039-versao-sobe-a-cada-pr-pelo-tipo-do-commit.md) (versão de desenvolvimento `X.Y.Z-dev.N`)

## Contexto

Quem desenvolve testa o `develop` instalado de verdade, em dois jeitos:

- **pelo `.msi` de desenvolvimento** (`FinancasLocal-desenvolvimento.msi`, versão `X.Y.Z-dev.N`), gerado a cada merge no `develop` depois de todos os testes de instalação (ADR 0029);
- **pelos scripts, num clone git** na branch `develop`.

Nos dois, pegar a versão mais nova exigia sair do gerenciador: baixar o `.msi` de novo à mão, ou abrir o terminal (`git pull` e os scripts). Isso contraria o motivo de o gerenciador existir, que é usar o sistema sem terminal (Nielsen 2, ADR 0028).

As ADRs 0028 e 0033 tinham fechado essa porta de propósito: código que ainda não saiu numa versão não deve chegar a quem usa, e no Windows o site roda como serviço com privilégio. O usuário decidiu (#154) que a atualização pelo `develop` é **só para quem desenvolve**: a instalação de uma versão publicada continua exatamente como antes.

## Alternativas consideradas

- **Manter só o comando para copiar** (0033) e o download manual do `.msi` de desenvolvimento. Prós: nada muda nas ADRs. Contras: quem desenvolve continua no terminal a cada merge.
- **Botão para todos, com o develop como opção** (interruptor "Usar a versão em desenvolvimento", como a issue propunha). Prós: um botão só. Contras: leva código não lançado a quem instalou uma versão publicada, que é o risco que a 0028 recusou.
- **Só na instalação de desenvolvimento**, reconhecida pelo que ela é: escolhida.

## Decisão

O gerenciador oferece **"Atualizar para a última do develop"** só quando a instalação **é** de desenvolvimento. A de uma versão publicada não ganha nada novo.

| Instalação | Como é reconhecida | O que o gerenciador faz |
|---|---|---|
| `.msi` de versão publicada (`X.Y.Z`) | versão sem `-dev` | como antes: procura só no Release mais recente (`releases/latest`) |
| `.msi` de desenvolvimento (`X.Y.Z-dev.N`) | versão com `-dev.N` | procura no pré-lançamento **"desenvolvimento"** (`releases/tags/desenvolvimento`), compara o `X.Y.Z` e o `N`, baixa o `.msi`, confere o SHA256 e abre o instalador, como na versão publicada |
| Scripts, num clone git na branch `develop` | `.git` na pasta do projeto + `git rev-parse --abbrev-ref HEAD` = `develop` | botão que roda `scripts/geral/atualizar.sh` (Linux) ou `scripts/windows/atualizar-windows.ps1` (Windows) |
| Scripts, fora do `develop` (tag, `main`, outra branch) ou sem git | — | como antes: só o texto ou o comando para atualizar à mão |

**Os scripts de atualizar** (a lógica fica neles, não no gerenciador: a CI testa e eles servem no terminal):
- recusam, sem mexer em nada, quando a pasta não é clone, o git não existe, a branch não é `develop`, há alterações locais ou há commits locais que não estão no `develop` do GitHub;
- só avançam (`git fetch` + `git merge --ff-only`), nunca descartam nada;
- saem com 0 (atualizado), 10 (já estava na última), 2 (recusado, com o motivo) ou 1 (erro).

**Windows:** o git roda **como o usuário** (`atualizar-windows.ps1 -SoCodigo`); só a reinstalação pede o UAC, com o próprio gerenciador reaberto com o argumento fixo `--instalar-pelos-scripts`, que calcula a pasta sozinho e não roda o git. A janela acompanha o log do instalador. **Linux:** o serviço é do usuário, então nada pede senha.

**O gerenciador não se troca enquanto está aberto:** no fim, o botão "Atualizar o gerenciador e reabrir" fecha, roda o `instalar-gerenciador.*` e abre de novo.

## Consequências

- **Positivas:** quem desenvolve atualiza o `develop` com um clique, nas duas janelas e nos dois tipos de instalação; quem usa uma versão publicada não vê nem recebe código não lançado.
- **Negativas:**
  - **Saída de rede nova no gerenciador do Linux:** o `git fetch` acessa o GitHub, mas só quando a pessoa clica, e só na instalação de desenvolvimento. A 0033 ("o gerenciador do Linux não acessa a internet") continua valendo para todo o resto.
  - **O gerenciador do Windows passa a consultar `releases/tags/desenvolvimento`** quando a versão instalada é de desenvolvimento. Mesmo domínio (`api.github.com`) e mesmo cuidado (só leitura, SHA256 conferido).
  - **Código não testado na instalação pelos scripts:** o `atualizar.*` traz o `develop` como está no GitHub. Ele passou pela CI do PR, mas não pelo teste do `.msi` que roda depois do merge. Aceito por ser só para quem desenvolve.
  - **Instalação de desenvolvimento pelo `.msi` não recebe a versão publicada:** ela só olha o pré-lançamento. Para voltar à versão publicada, é instalar o `.msi` dela por cima (a mesma `X.Y.Z` troca uma pela outra, ADR 0029).
  - **Mais uma ação com administrador no Windows** (`--instalar-pelos-scripts`), com argumento fixo e pasta calculada pelo próprio gerenciador.
- **Na prática:**
  - os workflows "Teste de instalação no Linux/Windows" rodam o `atualizar.*` no checkout da CI: fora do `develop` (PR) ele tem de recusar com 2; no push do `develop`, dizer 10; nos dois casos, sem mudar o commit nem os arquivos;
  - os dois gerenciadores têm as mesmas frases (CLAUDE.md: "Texto ou ação nova vai nas duas janelas").
