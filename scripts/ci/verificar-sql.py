"""Confere se toda criação de tabela, coluna ou índice em banco/01-estrutura.sql está protegida por IF
(os scripts do banco precisam ser idempotentes: o projeto não usa migrations)."""
import re
import sys

ARQUIVO = "banco/01-estrutura.sql"
CRIACAO = re.compile(r"^\s*(CREATE\s+TABLE|CREATE\s+(UNIQUE\s+)?INDEX|ALTER\s+TABLE\s+\S+\s+ADD\s)", re.IGNORECASE)
PROTECAO = re.compile(r"\bIF\b", re.IGNORECASE)

linhas = open(ARQUIVO, encoding="utf-8-sig").read().splitlines()
erros = []
for i, linha in enumerate(linhas):
    if not CRIACAO.match(linha):
        continue
    anterior = next((linhas[j] for j in range(i - 1, -1, -1) if linhas[j].strip()), "")
    if not PROTECAO.search(linha) and not PROTECAO.search(anterior):
        erros.append(f"{ARQUIVO}:{i + 1}: sem IF de proteção -> {linha.strip()}")

if erros:
    print("\n".join(erros))
    sys.exit(1)
print(f"OK: {ARQUIVO} idempotente.")
