# Operação: quando algo dá errado

| O que aconteceu | Vá para |
|---|---|
| Perdi dados, ou o banco ficou com problema | [Restaurar uma cópia](#restaurar-uma-cópia) |
| Troquei de computador, ou apaguei a pasta `chaves` | [Chaves perdidas](#chaves-perdidas) |
| Uma atualização não funcionou | [Atualização que falhou](#atualização-que-falhou) |

Fazer a cópia está em [Instalador e gerenciador](INSTALADOR-E-GERENCIADOR.md) (**Mais opções**) e, pelos scripts, no `scripts/geral/4-backup.sh`.

## Restaurar uma cópia

Os passos abaixo são testados nos workflows "Teste de instalação" a cada mudança nos scripts (#468).

### Antes de começar

**De onde vem a cópia:**

| Cópia | Onde fica | Quem faz |
|---|---|---|
| A que você salvou | Na pasta que você escolheu | Gerenciador, em **Mais opções** |
| Antes de cada atualização que muda o banco | `dados\backups\FinancasLocal-antes-atualizacao-*.db` (as 5 mais recentes) | O próprio sistema |
| Pelos scripts | `backups\FinancasLocal_*.db` e `backups\FinancasLocal-antes-instalacao-*.db`, na pasta do projeto | `4-backup.sh` e a instalação pelos scripts |

**O que muda ao restaurar:**
- Tudo volta para a data da cópia, **inclusive a senha do site**: entre com a senha que valia naquele dia.
- O que veio do Open Finance depois da cópia volta na próxima sincronização, dentro do período que ela busca. O que você lançou ou alterou à mão depois da cópia, não.
- Cópia de uma versão mais antiga: tudo bem, o sistema atualiza a estrutura ao iniciar.
- Cópia de uma versão **mais nova** que a instalada: atualize o sistema antes.

> **Não pule a remoção do `-wal` e do `-shm`.** O banco roda em modo WAL, e esses arquivos pertencem ao banco antigo. Se ficarem ao lado da cópia, [ela pode corromper](https://www.sqlite.org/howtocorrupt.html).

### No Windows, pelo `.msi`

Abra o **PowerShell como administrador** (a pasta de dados só abre para o sistema e os administradores), ajuste a primeira linha e rode:

```powershell
$copia = "E:\FinancasLocal-copia.db"
$dados = "$env:ProgramData\FinancasLocal\dados"

Stop-Service FinancasLocal
New-Item -ItemType Directory -Force "$dados\antes-de-restaurar" | Out-Null
Copy-Item "$dados\sql\FinancasLocal.db*" "$dados\antes-de-restaurar\"
Copy-Item $copia "$dados\sql\FinancasLocal.db" -Force
Remove-Item "$dados\sql\FinancasLocal.db-wal", "$dados\sql\FinancasLocal.db-shm" -ErrorAction SilentlyContinue
Start-Service FinancasLocal
```

Instalou pelos scripts? Troque o `$dados` pela pasta `dados` do projeto.

### No Linux, pelos scripts

Na pasta do projeto, ajuste a primeira linha e rode:

```bash
copia=/media/pendrive/FinancasLocal-copia.db

systemctl --user stop financaslocal.service
mkdir -p dados/antes-de-restaurar
cp dados/sql/FinancasLocal.db* dados/antes-de-restaurar/
cp "$copia" dados/sql/FinancasLocal.db
rm -f dados/sql/FinancasLocal.db-wal dados/sql/FinancasLocal.db-shm
systemctl --user start financaslocal.service
```

### Conferir

1. Abra o site e entre.
2. Veja se a última transação é a que você esperava.
3. Veja **Configurações › Erros**.

**Deu certo?** Apague a pasta `dados\antes-de-restaurar`. Ela tem o banco de antes, sem criptografia.

**Deu errado?** Repita os passos com `dados\antes-de-restaurar\FinancasLocal.db` no lugar da cópia. A pasta tem de ser copiada para fora antes, porque os comandos a sobrescrevem.

## Chaves perdidas

A pasta `dados\chaves` guarda as chaves que protegem os segredos gravados no banco. Elas se perdem se você apagar a pasta ou levar o banco para outro computador.

| Deixa de abrir | Continua funcionando |
|---|---|
| Client Secret da Pluggy e token da brapi | Todas as transações, contas, metas e investimentos, que não usam essas chaves |
| A sessão aberta no navegador: entre de novo | A senha do site, que fica no banco |

**O que fazer:** informe o Client Secret da Pluggy e o token da brapi de novo em **Configurações**. O sistema cria chaves novas sozinho.

**Instalou pelos scripts?** O certificado do site também fica nessa pasta (`dados/chaves/financaslocal.pfx`). Rode `scripts/geral/3-publicar-atualizar.sh` para criar outro. No `.msi`, ele fica em `dados\certificado`.

### Guardar as chaves junto da cópia?

**Não.** Com a chave ao lado do banco, quem pegar a cópia abre o Client Secret da Pluggy, e a proteção do segredo deixa de valer. Perder as chaves custa pouco: informar dois valores de novo. No Windows, elas nem abririam em outro computador, porque são protegidas pelo próprio Windows.

## Atualização que falhou

Nenhuma atualização apaga dados. Antes de mudar a estrutura do banco, o sistema guarda uma cópia em `dados\backups`.

### Pelo `.msi`

**O que acontece numa falha:** o instalador desfaz o que estava instalando. A versão antiga, porém, é removida antes de a nova entrar, e [essa remoção vem primeiro](https://learn.microsoft.com/en-us/windows/win32/msi/removeexistingproducts-action). Por isso, o site pode ficar sem nenhuma versão instalada. Seus dados continuam em `dados`.

**Onde ver o que houve:**

| Log | Onde |
|---|---|
| Instalador aberto pelo gerenciador | `%TEMP%\FinancasLocal\FinancasLocal-<versão>.log` |
| Instalador aberto à mão | Rode `msiexec /i FinancasLocal-<versão>.msi /l*v instalacao.log` |
| Erros do site | **Configurações › Erros** |
| Site que nem abre | **Visualizador de Eventos › Logs do Windows › Aplicativo** |

Os erros mais comuns do instalador estão em [Problemas comuns](PROBLEMAS-COMUNS.md#instalação-e-atualização-msi).

**Para voltar à versão anterior:**
1. Se ainda houver uma versão instalada, desinstale-a. Os dados ficam.
2. Instale o `.msi` da versão que você quer, na [página de versões](https://github.com/LucasDesenvGit/financas-local/releases).
3. Se a atualização chegou a mudar o banco e o site não abrir, [restaure](#restaurar-uma-cópia) a cópia mais recente de `dados\backups\FinancasLocal-antes-atualizacao-*.db`.

### Pelos scripts

- **Windows:** o `instalar-windows.cmd` volta sozinho para a versão anterior se a nova não subir. O log fica em `instaladores\instalacao-windows.log`.
- **Linux:** veja o que o script mostrou e o log do serviço, com `journalctl --user -u financaslocal.service`.
