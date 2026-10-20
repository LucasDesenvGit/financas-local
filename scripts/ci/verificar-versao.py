#!/usr/bin/env python3
"""
Todo pull request para o develop sobe a versão no Directory.Build.props (ADR 0039), pelo tipo do título
(Conventional Commits):
- feat → pelo menos o número do meio (2.1.3 → 2.2.0);
- fix, docs, build, ci, refactor, perf, test, chore, style → pelo menos o último (2.1.3 → 2.1.4);
- quebra de verdade (o usuário precisa fazer algo à mão ou não dá para voltar à versão anterior) → o primeiro
  (2.1.3 → 3.0.0). Fica a critério de quem faz o PR: feat(banco)! que só acrescenta tabela ou coluna é "feat".

Falha o PR se a versão não subiu ou subiu menos do que o tipo pede.

Uso (na CI): BASE=<branch de destino> TITULO_PR=<título do PR> python3 scripts/ci/verificar-versao.py
"""
import os
import re
import subprocess
import sys

ARQUIVO = 'Directory.Build.props'


def versao(conteudo: str) -> tuple[int, int, int]:
    achado = re.search(r'<Version>\s*(\d+)\.(\d+)\.(\d+)\s*</Version>', conteudo)
    if not achado:
        print(f'::error title=Versão::Não achei <Version>X.Y.Z</Version> no {ARQUIVO}.')
        sys.exit(1)
    return tuple(int(n) for n in achado.groups())


base = os.environ.get('BASE', 'develop')
titulo = os.environ.get('TITULO_PR', '')

anterior = subprocess.run(['git', 'show', f'origin/{base}:{ARQUIVO}'], capture_output=True, text=True, check=True).stdout
with open(ARQUIVO, encoding='utf-8') as f:
    atual = f.read()
de, para = versao(anterior), versao(atual)
texto = lambda v: '.'.join(map(str, v))

tipo = re.match(r'\s*([a-z]+)', titulo)
tipo = tipo.group(1) if tipo else ''
if tipo == 'feat':
    minima, nome = (de[0], de[1] + 1, 0), 'funcionalidade (feat): sobe o número do meio'
else:
    minima, nome = (de[0], de[1], de[2] + 1), 'correção ou ajuste: sobe o último número'

if para < minima:
    print(f'::error title=Versão::O {ARQUIVO} está em {texto(para)}; o {base} está em {texto(de)}. '
          f'Este PR é {nome}, então a versão tem de ser pelo menos {texto(minima)} (ADR 0039). '
          f'Se outro PR subiu a versão antes, pegue a maior e suba de novo.')
    sys.exit(1)
print(f'Versão {texto(de)} → {texto(para)} ({nome}).')
