# Documentação

Por onde começar: a [Arquitetura](ARQUITETURA.md), que vai da visão de cima até cada pasta. O [índice pelas seções do arc42](ARQUITETURA.md#índice-da-documentação) diz onde está cada assunto; esta página só lista os arquivos.

| Documento | Para quê |
|---|---|
| [ARQUITETURA.md](ARQUITETURA.md) | as partes do sistema, onde cada uma roda, o mapa de contextos e as pastas |
| [arquitetura/](arquitetura/) | os [componentes do Go](arquitetura/componentes.md), os [fluxos](arquitetura/fluxos.md) e o [domínio](arquitetura/dominio.md) (estados e termos) |
| [adr/](adr/README.md) | as decisões de arquitetura, com o porquê de cada uma |
| [BANCO-DE-DADOS.md](BANCO-DE-DADOS.md) | o modelo de dados e como mudar a estrutura sem migrations |
| [INSTALACAO-PELO-CODIGO.md](INSTALACAO-PELO-CODIGO.md) | instalar pelos scripts, desenvolver e migrar do SQL Server |
| [INSTALADOR-E-GERENCIADOR.md](INSTALADOR-E-GERENCIADOR.md) | as opções do `.msi` e do gerenciador |
| [OPERACAO.md](OPERACAO.md) e [PROBLEMAS-COMUNS.md](PROBLEMAS-COMUNS.md) | o que fazer quando algo dá errado: cópia, restauração, erros conhecidos |
| [SEGURANCA.md](SEGURANCA.md), [MODELO-DE-AMEACAS.md](MODELO-DE-AMEACAS.md) e [DADOS-PESSOAIS.md](DADOS-PESSOAIS.md) | o que sai para a internet, de quem o sistema se protege e o que ele guarda |
| [REQUISITOS-NAO-FUNCIONAIS.md](REQUISITOS-NAO-FUNCIONAIS.md) | as metas de memória, tamanho e partida, e como medir |
| [UI-UX.md](UI-UX.md) | as referências de UI/UX e os padrões adotados nas telas |

**Documento novo:** entra nesta tabela e no índice do arc42, no mesmo PR. `imagens/` guarda as figuras que os documentos usam.
