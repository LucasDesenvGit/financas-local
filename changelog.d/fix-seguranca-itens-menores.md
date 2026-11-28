### Correções
- **Instalação pelos scripts mais segura** (#68). No Linux, o serviço roda isolado e só grava na pasta `dados`. A instalação não executa mais scripts de pacotes do npm ao compilar o site.
  - Técnico: unidade do systemd com `ProtectSystem=strict` e `NoNewPrivileges`; `npm ci --ignore-scripts`; `SEGURANCA.md` corrige o que vai ao Yahoo.
