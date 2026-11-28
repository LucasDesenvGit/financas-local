### Correções
- **Mudar a segurança do login pede a senha** (#68). Em Configurações › Parametrização, mudar as tentativas antes do bloqueio, o tempo de bloqueio ou por quanto tempo o histórico de acessos fica guardado pede a sua senha atual. Quem achar o site aberto não consegue mais afrouxar o bloqueio.
  - Técnico: o PUT /api/parametros recebe `senhaAtual` e a confere quando muda um parâmetro com `pedeSenha`.
