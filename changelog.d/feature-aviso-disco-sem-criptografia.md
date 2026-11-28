### Funcionalidades
- **O gerenciador avisa quando os dados estão desprotegidos** (#68). Em “Mais opções”, ele avisa se o disco da pasta de dados está sem criptografia (BitLocker no Windows, LUKS no Linux) ou se ela está numa pasta sincronizada com a nuvem (OneDrive, Dropbox, Google Drive, iCloud), com um link que explica como resolver. Ele só avisa, e só quando tem certeza.
  - Técnico: pacote `internal/protecao` do gerenciador; BitLocker pelo `Shell.Application` (sem administrador), LUKS pelo `findmnt` e `lsblk -s`.
