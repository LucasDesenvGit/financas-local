# 0033. Gerenciador no Linux, com Avalonia e núcleo comum ao do Windows

- **Status:** Substituída por [0052](0052-gerenciador-em-go-com-bandeja-e-pagina-local.md): o gerenciador em C# saiu na #86 (fica na tag `v-ultimo-dotnet`). Antes, em parte substituída pela [0040](0040-atualizar-pelo-develop-so-na-instalacao-de-desenvolvimento.md) (na instalação de desenvolvimento, o botão roda o git)
- **Data:** 2026-10-18 (issue #138); encurtada em 2026-11-15 (#712)
- **Relacionadas:** [0028](0028-gerenciador-do-windows-e-atualizacao-pelo-github.md) (gerenciador do Windows)

## Contexto

No Linux, o site é um serviço do `systemd --user`, sem janela. A ADR 0028 deixou o gerenciador "só Windows", e o dele era WinForms, que não roda no Linux. Quem queria saber se o site estava no ar, ligá-lo, ver por que não subiu ou copiar o banco precisava do terminal (`systemctl --user`, `journalctl`).

No Linux, o serviço e o banco são do próprio usuário (nada pede elevação), não existe `.msi` (a versão nova vem pelos scripts) e o serviço só roda com a sessão aberta, a não ser com o *linger*.

## Alternativas consideradas

- **Avalonia só no Linux, com um núcleo comum aos dois.** **Escolhida:** o WinForms e o `.msi` não mudavam, e a lógica ficava num lugar só.
- **Avalonia nos dois sistemas:** uma tela só, mas refazia o visual recém-feito do Windows e mudava o `.msi`.
- **GTK (GtkSharp):** binding com pouca manutenção, sem ganho sobre o Avalonia.
- **Só um comando no terminal:** não resolve o "sem terminal", que é o motivo da 0028.

## Decisão

- **Três projetos:** um núcleo comum (`net10.0`: instalação, situação, cópia do banco e o serviço de cada sistema), a janela WinForms do Windows e a janela Avalonia do Linux, com a mesma ordem e as mesmas frases.
- **No Linux:** o serviço pelo `systemctl --user`, sem senha; a cópia do banco direta, com permissão `600`; "Ver o registro do serviço" com o `journalctl`; o aviso quando o *linger* está desligado; e, em vez de baixar a versão nova, o **comando de atualizar pronto para copiar**. Nenhuma saída de rede.
- **Instalação** pelo `scripts/instalar-gerenciador.sh`, sem root, e um modo `--situacao` que escreve a situação no terminal, para a CI.

## Consequências

- **Positivas:** a situação visível sem terminal, com as mesmas palavras do Windows, e o registro do serviço a um clique quando o site nem sobe.
- **Negativas:** o Avalonia como dependência (o pacote publicado tinha cerca de 100 MB, com o runtime do .NET), duas janelas para manter em paralelo, e sem ícone na bandeja do GNOME sem a extensão AppIndicator.
- **O que continuou no gerenciador em Go ([0052](0052-gerenciador-em-go-com-bandeja-e-pagina-local.md)):** um gerenciador só para os dois sistemas, com o serviço do usuário sem elevação, o aviso do *linger*, o `--situacao` e o comando de atualizar no Linux. A tabela de diferenças entre os sistemas e os detalhes do Avalonia ficaram na tag `v-ultimo-dotnet`.
