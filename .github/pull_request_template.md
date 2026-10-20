## O que muda

<!-- Resumo em 1-3 linhas. -->

## Tipo

- [ ] Funcionalidade (`feat`)
- [ ] Correção (`fix`)
- [ ] Refatoração / manutenção
- [ ] Documentação

## Checklist

- [ ] `dotnet build FinancasLocal.sln -c Release` sem erros
- [ ] `npm run build` em `src/frontend` sem erros
- [ ] `npm run test:tipos` e `npm test` em `src/frontend` passam (bug corrigido ganha teste de regressão)
- [ ] Mudou o banco? Bloco idempotente em `banco/01-estrutura.sql` + entidade e `FinancasDbContext` (sem migrations)
- [ ] Consultas só com LINQ/lambda
- [ ] Nenhum dado pessoal, senha, chave ou `appsettings.Production.json`
- [ ] README/CHANGELOG atualizados, se necessário
- [ ] **Manual do site** (`src/frontend/src/app/paginas/manual-conteudo.ts`) atualizado se mudou algo que o usuário vê; se não precisa, escreva aqui "Manual: não se aplica" (a CI avisa quando falta)

## Depois do merge

- [ ] Rodar `scripts/2-criar-banco.sh` (se mudou o banco)
- [ ] Rodar `scripts/3-publicar-atualizar.sh`
