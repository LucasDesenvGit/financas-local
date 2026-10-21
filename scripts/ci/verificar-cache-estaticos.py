#!/usr/bin/env python3
"""
#305: confere a regra que dá cache de um ano ("immutable") aos arquivos do build do Angular com hash no nome.

A regra fica em src/backend/Financas.Api/Program.cs (arquivoComHash) e é lida de lá, sem cópia que possa ficar
desatualizada. A regra antiga, -[A-Z0-9]{8}, só aceitava maiúsculas e números; o esbuild do Angular gera o hash com
minúsculas, "_" e "-" (chunk-BZ-xW7Tm.js), e só 3 dos 35 arquivos recebiam o cache longo.

Confere:
- nomes reais de um build do Angular (de 21/10/2026, esbuild 0.28) recebem o cache longo;
- o que não tem hash (index.html, favicon, manifesto, ícones, bandeiras) não recebe: ficaria um ano sem atualizar;
- se houver um build local (src/frontend/dist/financas-web/browser), todo .js/.css da raiz dele recebe.

Uso: python3 scripts/ci/verificar-cache-estaticos.py
"""
import os
import re
import sys

RAIZ = os.path.normpath(os.path.join(os.path.dirname(os.path.abspath(__file__)), '..', '..'))
PROGRAM = os.path.join(RAIZ, 'src', 'backend', 'Financas.Api', 'Program.cs')
BUILD = os.path.join(RAIZ, 'src', 'frontend', 'dist', 'financas-web', 'browser')

COM_HASH = [
    'main-SR6VQB6V.js', 'styles-5INURTSO.css', 'chunk-1dMxoNi3.js', 'chunk-CsAIQ8X_.js', 'chunk-BZ-xW7Tm.js',
    'chunk-_kYXmowP.js', 'chunk-pwoA-WoI.js', 'chunk-h1_N4G9s.js', 'chunk-N_OCWjPa.js', 'chunk-BcI9-NQX.js',
    'polyfills-FFHMD2TL.js',
]
SEM_HASH = [
    'index.html', 'favicon.svg', 'manifest.webmanifest', 'icone-192.png', 'icone-512.png', 'apple-touch-icon.png',
    'us.svg', 'btc.svg', 'LICENSE.txt', '3rdpartylicenses.txt',
]


def regra():
    texto = open(PROGRAM, encoding='utf-8').read()
    m = re.search(r'var arquivoComHash = new [\w.]*Regex\(@"([^"]+)"', texto)
    if not m:
        sys.exit('Não achei a regra arquivoComHash no Program.cs (mudou de nome? atualize este script).')
    return re.compile(m.group(1))


def main():
    padrao = regra()
    erros = [f'sem cache longo, mas tem hash: {n}' for n in COM_HASH if not padrao.search(n)]
    erros += [f'com cache longo, mas não tem hash (ficaria um ano sem atualizar): {n}' for n in SEM_HASH if padrao.search(n)]
    conferidos_build = 0
    if os.path.isdir(BUILD):
        for nome in sorted(os.listdir(BUILD)):
            if nome.endswith(('.js', '.css')):
                conferidos_build += 1
                if not padrao.search(nome):
                    erros.append(f'do build local, sem cache longo: {nome}')
    if erros:
        print(f'Regra do cache ({padrao.pattern}) errada:')
        for e in erros:
            print(f'  - {e}')
        return 1
    extra = f' e os {conferidos_build} .js/.css do build local' if conferidos_build else ''
    print(f'OK: regra do cache ({padrao.pattern}) certa para {len(COM_HASH)} nomes com hash, {len(SEM_HASH)} sem{extra}.')
    return 0


if __name__ == '__main__':
    sys.exit(main())
