#!/usr/bin/env python3
"""
#255 (ADR 0047): no PR, o .msi (versao.yml, ~7 min no Windows) só é gerado e testado quando o PR mexe no que vai
dentro dele ou no jeito de instalar. O Directory.Build.props muda em todo PR (a versão sobe, ADR 0039); se a única
mudança nele é a linha <Version>, ele não conta. No push do develop e na tag, o .msi roda sempre (o workflow não chama
este script).

Uso: python3 scripts/ci/precisa-msi.py --base origin/develop   (escreve msi=true|false em $GITHUB_OUTPUT)
"""
import argparse
import os
import re
import subprocess
import sys

RAIZ = os.path.normpath(os.path.join(os.path.dirname(os.path.abspath(__file__)), '..', '..'))
RELEVANTES = [
    r'^instalador/', r'^src/gerenciador/', r'^src/backend/Financas\.Api/Program\.cs$',
    r'^src/backend/Financas\.Api/Data/EsquemaBanco\.cs$', r'^\.github/workflows/versao\.yml$',
    r'^scripts/ci/precisa-msi\.py$',
]


def git(*args):
    return subprocess.run(['git', *args], cwd=RAIZ, capture_output=True, text=True, check=True).stdout


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--base', required=True)
    a = p.parse_args()
    ponto = git('merge-base', a.base, 'HEAD').strip()
    mudados = [l for l in git('diff', '--name-only', f'{ponto}...HEAD').splitlines() if l.strip()]
    motivos = [m for m in mudados if any(re.search(r, m) for r in RELEVANTES)]
    if 'Directory.Build.props' in mudados:
        linhas = [l for l in git('diff', '-U0', f'{ponto}...HEAD', '--', 'Directory.Build.props').splitlines()
                  if l[:1] in '+-' and not l.startswith(('+++', '---'))]
        if any('<Version>' not in l for l in linhas):
            motivos.append('Directory.Build.props (além da versão)')
    msi = bool(motivos)
    texto = ('## Instalador (.msi) neste PR\n\n' +
             (f'**Gera e testa**, porque mudou: {", ".join(motivos[:6])}.' if msi else
              'Pulado: o PR não mexe no instalador, no gerenciador nem na inicialização da API (só a versão no '
              'Directory.Build.props, que não conta). O .msi roda no push do develop e na tag.'))
    print(texto)
    if os.environ.get('GITHUB_OUTPUT'):
        with open(os.environ['GITHUB_OUTPUT'], 'a', encoding='utf-8') as f:
            f.write(f"msi={'true' if msi else 'false'}\n")
    if os.environ.get('GITHUB_STEP_SUMMARY'):
        with open(os.environ['GITHUB_STEP_SUMMARY'], 'a', encoding='utf-8') as f:
            f.write(texto + '\n')
    return 0


if __name__ == '__main__':
    sys.exit(main())
