#!/usr/bin/env python3
"""
Trava de desempenho dos testes do Jest (#204). Lê o resultado em JSON do Jest e:
- avisa (::warning) quando um arquivo de teste passa de AVISO_POR_ARQUIVO segundos;
- falha quando um arquivo passa de LIMITE_POR_ARQUIVO ou a suíte inteira passa de LIMITE_TOTAL.

Referência (20/10/2026, 2 workers, como o runner da CI): suíte em ~34 s, arquivo mais lento 6 s, mediana 0,7 s.
Antes do #204, um pacote pesado importado nos testes deixava cada arquivo em ~46 s e a suíte em ~350 s: os dois
limites teriam pego. Arquivo lento quase sempre é importação pesada (ver CONTRIBUINDO.md, "Testes rápidos").

Uso (na CI): npm test -- --ci --json --outputFile=jest-resultado.json
             python3 scripts/ci/verificar-tempo-testes.py src/frontend/jest-resultado.json
"""
import json
import sys

AVISO_POR_ARQUIVO = 10.0
LIMITE_POR_ARQUIVO = 20.0
LIMITE_TOTAL = 150.0

caminho = sys.argv[1] if len(sys.argv) > 1 else 'src/frontend/jest-resultado.json'
with open(caminho, encoding='utf-8') as f:
    resultado = json.load(f)

arquivos = []
for t in resultado['testResults']:
    nome = t['name'].replace('\\', '/')
    nome = nome[nome.find('testes/'):] if 'testes/' in nome else nome
    arquivos.append((nome, (t['endTime'] - t['startTime']) / 1000))
arquivos.sort(key=lambda a: -a[1])

fim = max(t['endTime'] for t in resultado['testResults'])
total = (fim - resultado['startTime']) / 1000

falhou = False
for nome, segundos in arquivos:
    if segundos > LIMITE_POR_ARQUIVO:
        print(f'::error title=Teste lento::{nome} levou {segundos:.1f} s (limite {LIMITE_POR_ARQUIVO:.0f} s). '
              'Quase sempre é uma importação pesada: veja "Testes rápidos" no CONTRIBUINDO.md.')
        falhou = True
    elif segundos > AVISO_POR_ARQUIVO:
        print(f'::warning title=Teste lento::{nome} levou {segundos:.1f} s (aviso a partir de {AVISO_POR_ARQUIVO:.0f} s).')

if total > LIMITE_TOTAL:
    print(f'::error title=Jest lento::a suíte levou {total:.0f} s (limite {LIMITE_TOTAL:.0f} s). '
          'Veja os arquivos mais lentos abaixo e "Testes rápidos" no CONTRIBUINDO.md.')
    falhou = True

print(f'Jest: {len(arquivos)} arquivos em {total:.0f} s. Mais lentos:')
for nome, segundos in arquivos[:5]:
    print(f'  {segundos:5.1f} s  {nome}')
sys.exit(1 if falhou else 0)
