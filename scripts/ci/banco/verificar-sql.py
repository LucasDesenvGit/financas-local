"""Confere se toda criação de tabela/índice em banco/*.sql usa IF NOT EXISTS e se não
sobrou sintaxe exclusiva de T-SQL (os scripts do banco precisam ser idempotentes e
compatíveis com SQLite: o projeto não usa migrations)."""
import glob
import re
import sys

ARQUIVOS = sorted(glob.glob("banco/*.sql"))

CRIACAO_TABELA = re.compile(r"^\s*CREATE\s+TABLE\s+(?!IF\s+NOT\s+EXISTS\b)", re.IGNORECASE)
CRIACAO_INDICE = re.compile(r"^\s*CREATE\s+(UNIQUE\s+)?INDEX\s+(?!IF\s+NOT\s+EXISTS\b)", re.IGNORECASE)

# Tokens que só existem em T-SQL/SQL Server e não devem sobrar depois da migração para SQLite.
TOKENS_TSQL = [
    (re.compile(r"^\s*GO\s*$", re.IGNORECASE), "'GO' é separador de lote do sqlcmd/SSMS; SQLite não usa (statements terminam em ';')"),
    (re.compile(r"\bdbo\."), "prefixo de schema 'dbo.' não existe em SQLite"),
    (re.compile(r"\[[^\]\r\n]*\]"), "identificador entre colchetes '[...]' é sintaxe do SQL Server"),
    (re.compile(r"\bIDENTITY\s*\(", re.IGNORECASE), "'IDENTITY(...)' não existe em SQLite (use INTEGER PRIMARY KEY AUTOINCREMENT)"),
    (re.compile(r"\bNEWID\s*\(", re.IGNORECASE), "'NEWID()' não existe em SQLite (o Guid já é gerado pela API)"),
    (re.compile(r"\bSYSUTCDATETIME\s*\(", re.IGNORECASE), "'SYSUTCDATETIME()' não existe em SQLite (o timestamp já é gerado pela API)"),
    (re.compile(r"\bUNIQUEIDENTIFIER\b", re.IGNORECASE), "'UNIQUEIDENTIFIER' não existe em SQLite (use TEXT)"),
    (re.compile(r"(?<![A-Za-z0-9_])[nN]'"), "prefixo de string nacional \"N'...'\" é desnecessário em SQLite"),
    (re.compile(r"\bOBJECT_ID\s*\(", re.IGNORECASE), "'OBJECT_ID(...)' não existe em SQLite (use CREATE ... IF NOT EXISTS)"),
    (re.compile(r"\bCOL_LENGTH\s*\(", re.IGNORECASE), "'COL_LENGTH(...)' não existe em SQLite"),
]

erros = []
for arquivo in ARQUIVOS:
    linhas = open(arquivo, encoding="utf-8-sig").read().splitlines()
    for i, linha in enumerate(linhas):
        if CRIACAO_TABELA.match(linha):
            erros.append(f"{arquivo}:{i + 1}: CREATE TABLE sem IF NOT EXISTS -> {linha.strip()}")
        if CRIACAO_INDICE.match(linha):
            erros.append(f"{arquivo}:{i + 1}: CREATE INDEX sem IF NOT EXISTS -> {linha.strip()}")
        for padrao, motivo in TOKENS_TSQL:
            if padrao.search(linha):
                erros.append(f"{arquivo}:{i + 1}: {motivo} -> {linha.strip()}")

if erros:
    print("\n".join(erros))
    sys.exit(1)
print(f"OK: {', '.join(ARQUIVOS)} idempotentes e sem sintaxe de T-SQL.")
