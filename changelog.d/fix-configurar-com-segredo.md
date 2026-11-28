### Correções
- **Primeiro usuário só pelo link da instalação** (#68). O site só deixa criar o usuário pelo link que o gerenciador abre ou que a instalação mostra (vale 10 minutos), e outro programa do computador não o cria antes de você.
  - Técnico: `POST /api/auth/configurar` exige `X-Configurar-Token` (HMAC do segredo do gerenciador); subcomando `link-de-configuracao`; ADR 0075.
