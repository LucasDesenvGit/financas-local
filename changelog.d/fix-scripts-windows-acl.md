### Correções
- **Instalação pelos scripts no Windows protege as pastas do serviço** (#68). Só o sistema e os administradores mudam o programa publicado, o banco, as chaves e as cópias, como no instalador `.msi`. Depois de instalar, criar o banco e fazer cópias pelo Git Bash pedem abri-lo como Administrador; o backup agendado passa a rodar com privilégios mais altos.
  - Técnico: `Proteger-Pasta` no `scripts/windows/comum.ps1`, usado também pelo `configurar.ps1`; ADR 0012 atualizada.
