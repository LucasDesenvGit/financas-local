# Instalador e gerenciador do Windows

O caminho básico (instalar, atualizar e desinstalar) está no [README](../README.md#instalar-no-windows). Aqui ficam as opções a mais e tudo o que o gerenciador faz.

## Opções da instalação

- **Conferir o arquivo:** baixe também o `SHA256SUMS.txt` e rode `Get-FileHash .\FinancasLocal-<versão>.msi` no PowerShell. O resultado tem de ser igual ao número do arquivo.
- **Sem o atalho na área de trabalho:** `msiexec /i FinancasLocal-<versão>.msi ATALHOAREADETRABALHO=0`.
- **Versão de desenvolvimento:** quer testar o que já foi aprovado e ainda não saiu? Baixe o `FinancasLocal-desenvolvimento.msi` do [pré-lançamento "desenvolvimento"](https://github.com/LucasDesenvGit/financas-local/releases/tag/desenvolvimento). Ele é gerado a cada mudança aprovada, instala e atualiza igual e mantém os dados. O site mostra a versão como `X.Y.Z-dev.N`, e o gerenciador dessa instalação avisa e atualiza para a versão de desenvolvimento mais nova (quem instalou uma versão publicada continua recebendo só as publicadas).

O site liga sozinho com o computador e só responde nele mesmo. No Menu Iniciar ficam os atalhos **Finanças Local** (o site) e **Gerenciador do Finanças Local**, que também fica na área de trabalho.

## O gerenciador

- Mostra se o site está **no ar** (com a versão), **desligado** ou **ligado, mas sem responder**.
- **Abrir o site**, **Ligar/Desligar** e **Reiniciar** (as duas últimas pedem permissão de administrador).
- **Versão nova:** avisa, mostra **O que mudou** e atualiza com um clique.
- **Mais opções:** salvar uma cópia do banco onde você escolher (pendrive, nuvem…), ver os erros do site, abrir o manual e **iniciar com o Windows** (fica na área de notificação e avisa se o site cair).
- **Botão direito no ícone da área de notificação:** mostra a situação do site ("Finanças Local · No ar") e tem **Abrir o site**, **Mostrar o gerenciador** e **Sair**, nas cores do site.

## Atualizar

Nenhuma atualização apaga dados. Antes de mudar a estrutura do banco, a versão nova guarda uma cópia em `dados\backups`. A versão instalada aparece no gerenciador e no site, em **Configurações › Parametrização › Sistema**.

- **Pelo gerenciador:** quando aparecer **"Tem versão nova"**, veja **O que mudou** e clique em **Atualizar**. Ele baixa, confere e abre o instalador; é só **Avançar**, **Instalar** e **Sim**. O site fica fora do ar por alguns segundos.
- **Sem o gerenciador:** baixe o `.msi` novo da [versão mais recente](https://github.com/LucasDesenvGit/financas-local/releases/latest) e abra. Ele troca a versão instalada e mantém os dados.
- **Voltar a uma versão anterior:** desinstale a atual antes (os dados ficam) e instale o `.msi` da versão que quer.
- **Instalou pelos scripts?** Veja [Passar para o instalador `.msi`](INSTALACAO-PELO-CODIGO.md#instalado-pelos-scripts-e-quer-passar-para-o-instalador).

## Desinstalar

Em **Configurações do Windows › Aplicativos › Aplicativos instalados › Finanças Local › Desinstalar**. Saem o programa, o serviço e o certificado; **os seus dados ficam** em `C:\ProgramData\FinancasLocal\dados`.

- **Para apagar os dados também:** `msiexec /x FinancasLocal-<versão>.msi APAGARDADOS=1`.
- **Para só desligar o site por um tempo:** use **Desligar** no gerenciador.
