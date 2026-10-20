#!/usr/bin/env python3
"""
Avisa (sem bloquear) quando um pull request mexe no que o usuário vê, telas do front-end ou endpoints da API, sem
atualizar o Manual do site (#177). O Manual fica em src/frontend/src/app/paginas/manual-conteudo.ts.

Dispensa: escrever "Manual: não se aplica" (ou "Manual: nao se aplica") na descrição do PR, para bugfix ou refatoração
que não muda o que o usuário vê.

Uso (na CI): BASE=<branch de destino> CORPO_PR=<descrição do PR> python3 scripts/ci/verificar-manual.py
"""
import os
import re
import subprocess
import sys

MANUAL = 'src/frontend/src/app/paginas/manual-conteudo.ts'
VISIVEIS = ('src/frontend/src/app/paginas/', 'src/frontend/src/app/compartilhado/', 'src/backend/Financas.Api/Controllers/')
IGNORAR = (MANUAL, 'src/frontend/src/app/paginas/manual.component.ts')

base = os.environ.get('BASE', 'develop')
corpo = os.environ.get('CORPO_PR', '')

if re.search(r'manual:\s*n[aã]o se aplica', corpo, re.IGNORECASE):
    print('Manual: não se aplica (dito na descrição do PR).')
    sys.exit(0)

alterados = subprocess.run(['git', 'diff', '--name-only', f'origin/{base}...HEAD'], capture_output=True, text=True,
                           check=False).stdout.split()
if MANUAL in alterados:
    print('Manual atualizado neste PR.')
    sys.exit(0)

visiveis = [a for a in alterados if a.startswith(VISIVEIS) and a not in IGNORAR and not a.endswith('.spec.ts')]
if not visiveis:
    print('Nenhuma tela ou endpoint mudou: o Manual não precisa mudar.')
    sys.exit(0)

lista = ', '.join(visiveis[:5]) + (' …' if len(visiveis) > 5 else '')
print(f'::warning title=Manual do site::Este PR mexe no que o usuário vê ({lista}) e não atualiza o Manual '
      f'({MANUAL}). Atualize a seção da tela ou escreva "Manual: não se aplica" na descrição do PR.')
sys.exit(0)
