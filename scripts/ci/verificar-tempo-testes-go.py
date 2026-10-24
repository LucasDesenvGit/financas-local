#!/usr/bin/env python3
"""
Trava de tempo dos testes do Go (#371), como a do Jest (#204, verificar-tempo-testes.py). Lê a saída do go test -json e:
- mostra a saída dos testes e pacotes que falharam (com -json, o go test não a mostra como texto);
- avisa (::warning) quando um teste passa de AVISO_POR_TESTE segundos;
- falha quando um teste passa de LIMITE_POR_TESTE, um pacote passa de LIMITE_POR_PACOTE ou a suíte (do primeiro ao
  último evento, com os pacotes em paralelo) passa de LIMITE_TOTAL;
- escreve os 10 testes mais lentos no resumo da execução (GITHUB_STEP_SUMMARY).

Referência (24/10/2026, CI do develop, com -race e sem cobertura): a etapa de testes em 22 s; os pacotes mais lentos,
banco (20 s), autenticacao (17 s) e parametros (10 s). Os limites são *julgamento*: ~3 vezes o que se tem hoje.
Teste lento quase sempre é time.Sleep, banco recriado em cada subteste ou rede (CONTRIBUINDO.md, "Testes rápidos").

Uso (na CI, em src/backend-go):
  go test -race -json ./... > testes.json
  python3 ../../scripts/ci/verificar-tempo-testes-go.py testes.json
"""
import json
import os
import re
import sys
from collections import defaultdict
from datetime import datetime

AVISO_POR_TESTE = 5.0
LIMITE_POR_TESTE = 20.0
LIMITE_POR_PACOTE = 60.0
LIMITE_TOTAL = 120.0

MODULO = 'github.com/LucasDesenvGit/financas-local/src/backend-go/'


def instante(texto):
    # O Go escreve até 9 casas na fração; o fromisoformat do Python aceita até 6.
    data, fracao, fuso = re.fullmatch(r'(.{19})(?:\.(\d+))?(Z|[+-]\d\d:\d\d)', texto).groups()
    return datetime.fromisoformat(f'{data}.{(fracao or "0")[:6].ljust(6, "0")}{"+00:00" if fuso == "Z" else fuso}')


def main():
    sys.stdout.reconfigure(encoding='utf-8')  # o console do Windows não é UTF-8
    caminho = sys.argv[1] if len(sys.argv) > 1 else 'testes.json'
    testes, pacotes, saidas, falharam = {}, {}, defaultdict(list), []
    primeiro = ultimo = None
    with open(caminho, encoding='utf-8') as f:
        for linha in f:
            if not linha.startswith('{'):
                print(linha, end='')  # erro de compilação e afins vêm como texto
                continue
            e = json.loads(linha)
            pacote = e.get('Package', '').removeprefix(MODULO)
            chave = (pacote, e.get('Test'))
            if 'Time' in e:
                t = instante(e['Time'])
                primeiro, ultimo = min(primeiro or t, t), max(ultimo or t, t)
            acao = e.get('Action')
            if acao == 'build-output':
                print(e.get('Output', ''), end='')  # erro de compilação (Go 1.24+: vem no JSON)
            elif acao == 'output':
                saidas[chave].append(e.get('Output', ''))
            elif acao in ('pass', 'fail'):
                if e.get('Test'):
                    testes[chave] = e.get('Elapsed', 0.0)
                else:
                    pacotes[pacote] = e.get('Elapsed', 0.0)
                if acao == 'fail':
                    falharam.append(chave)

    for pacote, teste in falharam:
        if teste and '/' in teste and (pacote, teste.split('/')[0]) in falharam:
            continue  # o subteste sai junto com o teste de cima
        print(f'::group::FALHOU {pacote} {teste or "(o pacote)"}')
        print(''.join(saidas[(pacote, teste)]), end='')
        print('::endgroup::')
        print(f'::error title=Teste do Go falhou::{pacote} {teste or ""}'.rstrip())

    falhou = False
    lentos = sorted(testes.items(), key=lambda t: -t[1])
    for (pacote, teste), segundos in lentos:
        if segundos > LIMITE_POR_TESTE:
            print(f'::error title=Teste lento::{pacote} {teste} levou {segundos:.1f} s (limite {LIMITE_POR_TESTE:.0f} s). '
                  'Quase sempre é time.Sleep, banco recriado em cada subteste ou rede: veja "Testes rápidos" no '
                  'CONTRIBUINDO.md.')
            falhou = True
        elif segundos > AVISO_POR_TESTE:
            print(f'::warning title=Teste lento::{pacote} {teste} levou {segundos:.1f} s '
                  f'(aviso a partir de {AVISO_POR_TESTE:.0f} s).')
    for pacote, segundos in sorted(pacotes.items(), key=lambda p: -p[1]):
        if segundos > LIMITE_POR_PACOTE:
            mais_lentos = ', '.join(f'{t} ({s:.1f} s)' for (p, t), s in lentos if p == pacote and '/' not in t)
            print(f'::error title=Pacote lento::{pacote} levou {segundos:.0f} s (limite {LIMITE_POR_PACOTE:.0f} s). '
                  f'Mais lentos: {mais_lentos[:300]}')
            falhou = True
    total = (ultimo - primeiro).total_seconds() if primeiro else 0.0
    if total > LIMITE_TOTAL:
        print(f'::error title=Testes do Go lentos::a suíte levou {total:.0f} s (limite {LIMITE_TOTAL:.0f} s). '
              'Veja os testes mais lentos no resumo e "Testes rápidos" no CONTRIBUINDO.md.')
        falhou = True

    resumo = ['### Tempo dos testes do Go (#371)', '',
              f'{len(testes)} testes em {len(pacotes)} pacotes, **{total:.0f} s** do primeiro ao último. Limites: '
              f'teste {LIMITE_POR_TESTE:.0f} s (aviso em {AVISO_POR_TESTE:.0f} s), pacote {LIMITE_POR_PACOTE:.0f} s, '
              f'suíte {LIMITE_TOTAL:.0f} s.', '',
              '| Teste | Pacote | Tempo |', '|---|---|---|',
              *[f'| `{t}` | `{p}` | {s:.2f} s |' for (p, t), s in lentos[:10]], '',
              '| Pacote | Tempo |', '|---|---|',
              *[f'| `{p}` | {s:.1f} s |' for p, s in sorted(pacotes.items(), key=lambda p: -p[1])[:5]], '']
    print('\n'.join(resumo))
    if os.environ.get('GITHUB_STEP_SUMMARY'):
        with open(os.environ['GITHUB_STEP_SUMMARY'], 'a', encoding='utf-8') as f:
            f.write('\n'.join(resumo) + '\n')
    sys.exit(1 if falhou or falharam else 0)


if __name__ == '__main__':
    main()
