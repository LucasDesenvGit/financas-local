# 0054. Análise de segurança do código com o CodeQL

- **Status:** Aceita
- **Data:** 2026-10-29
- **Relacionadas:** [0047](0047-ci-paralela-so-testes-afetados-no-pr.md), [0049](0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md), [0052](0052-gerenciador-em-go-com-bandeja-e-pagina-local.md)

## Contexto

As regras de segurança do código do projeto (`CLAUDE.md`) só são conferidas na revisão:
- SQL só parametrizado, nunca com texto do usuário (C# e Go);
- comando do sistema sem shell (`internal/comando`);
- caminho de arquivo vindo de fora (o destino da cópia do banco);
- nada de HTML sem escape no front-end.

O que a CI já tem é outra coisa. O `govulncheck` (job "Go") procura falha conhecida nas **bibliotecas** do Go, e o Dependabot, nas dependências. Nada olha o **nosso** código.

Os workflows do GitHub Actions também são código. Um `${{ github.event.pull_request.title }}` dentro de um `run:` deixa quem abre o PR rodar comando no runner. O projeto já passa esses valores por `env:` (por exemplo, `verificar-versao.py`), mas por costume, sem nada que confira.

O repositório é público, então o CodeQL do GitHub sai de graça ([sobre o code scanning](https://docs.github.com/en/code-security/code-scanning/introduction-to-code-scanning/about-code-scanning-with-codeql)). Pedido do @JulioCezarSP (#451).

## Alternativas consideradas

- **Só o `govulncheck` e a revisão (como estava):** custo zero. Mas não olha o nosso código, só o Go, e depende de quem revisa lembrar de cada regra.
- **CodeQL pelo "Default setup"** (ligado nas configurações do repositório, sem arquivo): um clique. Mas não deixa compilar o Go à mão, e o gerenciador em Go só compila no Linux com GTK 4, WebKitGTK e cgo (ADR 0052). Também tira do repositório a decisão de quando e como roda.
- **Semgrep ou outro analisador de fora:** regras fáceis de escrever. Mas é mais uma ferramenta e mais um serviço na CI, e a análise de fluxo de dados entre funções da versão gratuita é mais limitada que a do CodeQL. Os achados não caem no Code scanning do GitHub sem passo extra.
- **CodeQL por workflow próprio (escolhida):** C#, Go, TypeScript, Python e os workflows, com o Go compilado à mão.

## Decisão

Usamos o **CodeQL por workflow próprio**, `.github/workflows/codeql.yml`, com uma linguagem por job: `actions`, `csharp`, `go`, `javascript-typescript` e `python`.
- **O Go é compilado à mão** (`build-mode: manual`): os dois módulos, com o GTK 4 e o WebKitGTK instalados para o gerenciador. O autobuild não instala nada disso e analisaria só parte do código.
- **Quando roda:** em todo PR para o `develop` e o `main`, no push dos dois, toda quinta (as consultas ganham regras novas sem o código mudar) e à mão.
- **Os achados não bloqueiam o merge:** vão para Security › Code scanning e, no PR, viram anotações nas linhas.

## Consequências

- **Positivas:**
  - as regras de SQL, comando, caminho de arquivo e HTML passam a ser conferidas em todo PR, e não só na revisão;
  - injeção nos workflows é apontada no PR que a cria;
  - fica um histórico dos achados no Code scanning.
- **Negativas:**
  - **Tempo de CI:** são 5 jobs a mais por PR, em paralelo. Eles não somam ao tempo do `ci.yml`, mas ocupam runners. O do Go é o mais lento, porque instala o GTK e compila os dois módulos.
    - No PR, roda tudo, e não só o que o PR mexe, ao contrário do `ci.yml` (ADR 0047). Uma linguagem que faltasse no PR deixaria o Code scanning sem comparação com o `develop`. Se o tempo pesar, a saída é rodar no PR só o que mudou e aceitar esse aviso.
  - **Falso positivo:** alerta que não é falha precisa ser fechado à mão, com o motivo ("usado em teste", "falso positivo"), em Security › Code scanning.
  - **Mais uma ação de terceiros** na CI (`github/codeql-action`, do próprio GitHub), com permissão `security-events: write` só no job dela.
- **Na prática:**
  - **PR com anotação do CodeQL:** corrija, ou feche o alerta com o motivo. Não ignore em silêncio.
  - **Linguagem nova no projeto:** entra na matriz do `codeql.yml`.
  - **Mudou a compilação do Go** (módulo novo, biblioteca de sistema nova): o passo "Compilar o Go" do `codeql.yml` acompanha.
  - **Para barrar o merge com alerta grave:** é uma regra do repositório (*rulesets*, "Require code scanning results"), decidida à parte.
