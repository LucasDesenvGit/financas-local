#!/usr/bin/env python3
"""
Catraca da cobertura do Go (#370, ADR 0051). Lê o perfil do go test -coverpkg=./internal/... e o arquivo de pisos
(src/backend-go/cobertura-minima.txt) e:
- falha quando um pacote fica abaixo do piso dele, mostrando as funções que nenhum teste roda;
- falha quando um pacote novo (sem linha no arquivo) fica abaixo de PISO_PACOTE_NOVO;
- falha quando o PR desce um piso ou troca um piso por "fora" (--base: o arquivo do develop). O piso só sobe;
- avisa quando a cobertura passou do piso em 1 ponto ou mais: suba o piso no mesmo PR;
- escreve a tabela por pacote no resumo da execução (GITHUB_STEP_SUMMARY).

A cobertura é de linha executada, não de regra conferida: por isso existem a mutação à mão e a regra "teste novo
prova que falha" da ADR 0051.

Uso (na CI, em src/backend-go):
  go test -race -coverpkg=./internal/... -coverprofile=cobertura.out ./...
  python3 ../../scripts/ci/verificar-cobertura-go.py cobertura.out cobertura-minima.txt [--base pisos-do-develop.txt]
"""
import argparse
import math
import os
import subprocess
import sys
from collections import defaultdict

MODULO = 'github.com/LucasDesenvGit/financas-local/src/backend-go/'
PISO_PACOTE_NOVO = 80.0  # julgamento (#370): o pacote novo nasce com teste; a ADR 0051 registra
AVISO_PARA_SUBIR = 1.0


def ler_pisos(caminho):
    """pacote -> número (o piso) ou ('fora', motivo)."""
    pisos = {}
    with open(caminho, encoding='utf-8') as f:
        for n, linha in enumerate(f, 1):
            linha = linha.split('#', 1)[0].strip()
            if not linha:
                continue
            partes = linha.split(None, 2)
            if len(partes) >= 2 and partes[1] == 'fora':
                if len(partes) < 3:
                    sys.exit(f'{caminho}:{n}: "fora" precisa do motivo na mesma linha')
                pisos[partes[0]] = ('fora', partes[2])
            elif len(partes) == 2:
                pisos[partes[0]] = float(partes[1].replace(',', '.'))
            else:
                sys.exit(f'{caminho}:{n}: esperado "pacote piso" ou "pacote fora motivo", veio {linha!r}')
    return pisos


def ler_perfil(caminho):
    """pacote -> (instruções, instruções cobertas). O mesmo bloco aparece uma vez por pacote de teste: vale se algum
    deles o rodou."""
    blocos = {}
    with open(caminho, encoding='utf-8') as f:
        for linha in f:
            if linha.startswith('mode:') or not linha.strip():
                continue
            local, instrucoes, vezes = linha.rsplit(' ', 2)
            antes = blocos.get(local, (int(instrucoes), False))
            blocos[local] = (antes[0], antes[1] or int(vezes) > 0)
    pacotes = defaultdict(lambda: [0, 0])
    for local, (instrucoes, coberto) in blocos.items():
        arquivo = local.split(':', 1)[0]
        pacote = arquivo.removeprefix(MODULO).rsplit('/', 1)[0].removeprefix('internal/')
        pacotes[pacote][0] += instrucoes
        if coberto:
            pacotes[pacote][1] += instrucoes
    return {p: (t, c) for p, (t, c) in pacotes.items() if t > 0}


def funcoes_sem_teste(perfil, pacote, limite=15):
    """As funções do pacote com 0% (go tool cover -func), para dizer onde falta teste."""
    try:
        saida = subprocess.run(['go', 'tool', 'cover', f'-func={perfil}'], capture_output=True, text=True,
                               check=True).stdout
    except (OSError, subprocess.CalledProcessError):
        return []
    prefixo = MODULO + ('' if pacote.startswith('cmd/') else 'internal/') + pacote + '/'
    funcoes = []
    for linha in saida.splitlines():
        partes = linha.split()
        if len(partes) == 3 and partes[0].startswith(prefixo) and '/' not in partes[0][len(prefixo):].split(':')[0] \
                and partes[2] == '0.0%':
            funcoes.append(f'{partes[0].removeprefix(MODULO)} {partes[1]}')
    return funcoes[:limite]


def main():
    args = argparse.ArgumentParser()
    args.add_argument('perfil')
    args.add_argument('pisos')
    args.add_argument('--base', help='o arquivo de pisos do develop, para conferir que nenhum piso desceu')
    a = args.parse_args()
    sys.stdout.reconfigure(encoding='utf-8')  # o console do Windows não é UTF-8
    arquivo_pisos = os.path.basename(a.pisos)

    pisos = ler_pisos(a.pisos)
    cobertura = ler_perfil(a.perfil)
    falhou = False
    linhas = []

    if a.base and os.path.exists(a.base):
        for pacote, antes in ler_pisos(a.base).items():
            agora = pisos.get(pacote)
            if isinstance(antes, float) and (agora is None or isinstance(agora, tuple) or agora < antes):
                depois = 'sem linha' if agora is None else ('fora' if isinstance(agora, tuple) else f'{agora:.1f}')
                print(f'::error title=Piso de cobertura desceu::{pacote}: o piso era {antes:.1f} no develop e ficou '
                      f'{depois}. O piso só sobe (#370): ponha o teste de volta ou explique no PR e na ADR 0051.')
                falhou = True

    for pacote in sorted(set(cobertura) | {p for p, v in pisos.items() if isinstance(v, float)}):
        piso = pisos.get(pacote, PISO_PACOTE_NOVO)
        if pacote not in cobertura:
            print(f'::warning title=Pacote sem código::{pacote} tem piso em {arquivo_pisos}, mas não aparece na cobertura. '
                  'Foi apagado ou renomeado? Tire ou troque a linha.')
            continue
        total, cobertas = cobertura[pacote]
        pct = math.floor(1000 * cobertas / total) / 10  # uma casa, para baixo: o piso é o que já se tem
        if isinstance(piso, tuple):
            linhas.append(f'| `{pacote}` | {pct:.1f}% | fora | {piso[1]} |')
            continue
        novo = pacote not in pisos
        if pct < piso:
            falhou = True
            qual = f'pacote novo: o mínimo é {PISO_PACOTE_NOVO:.0f}%' if novo else f'o piso é {piso:.1f}%'
            print(f'::error title=Cobertura abaixo do piso::{pacote}: {pct:.1f}% ({qual}).')
            sem_teste = funcoes_sem_teste(a.perfil, pacote)
            if sem_teste:
                print(f'  Funções de {pacote} que nenhum teste roda:')
                for f in sem_teste:
                    print(f'    {f}')
            situacao = '❌ abaixo'
        elif novo:
            print(f'::warning title=Pacote novo sem piso::{pacote}: {pct:.1f}%. Ponha "{pacote} {pct:.1f}" em {arquivo_pisos}.')
            situacao = '⚠️ sem piso'
        elif pct >= piso + AVISO_PARA_SUBIR:
            print(f'::notice title=Suba o piso::{pacote}: {pct:.1f}% com piso {piso:.1f}. Suba para {pct:.1f} em '
                  f'{arquivo_pisos}, para a catraca não deixar descer de novo.')
            situacao = '⬆️ suba o piso'
        else:
            situacao = '✅'
        linhas.append(f'| `{pacote}` | {pct:.1f}% | {piso:.1f}% | {situacao} |')

    total = sum(t for p, (t, _) in cobertura.items() if not isinstance(pisos.get(p), tuple))
    cobertas = sum(c for p, (_, c) in cobertura.items() if not isinstance(pisos.get(p), tuple))
    geral = f'{100 * cobertas / total:.1f}%' if total else '–'
    resumo = ['### Cobertura do Go (#370)', '',
              f'Total, sem os pacotes "fora": **{geral}**. Piso por pacote em `src/backend-go/cobertura-minima.txt`; '
              f'pacote novo nasce com {PISO_PACOTE_NOVO:.0f}%.', '',
              '| Pacote | Cobertura | Piso | Situação |', '|---|---|---|---|', *linhas, '']
    print('\n'.join(resumo))
    if os.environ.get('GITHUB_STEP_SUMMARY'):
        with open(os.environ['GITHUB_STEP_SUMMARY'], 'a', encoding='utf-8') as f:
            f.write('\n'.join(resumo) + '\n')
    sys.exit(1 if falhou else 0)


if __name__ == '__main__':
    main()
