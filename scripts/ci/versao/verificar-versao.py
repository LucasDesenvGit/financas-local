#!/usr/bin/env python3
"""
Todo pull request para o develop sobe a versão no Directory.Build.props (ADR 0039), pelo tipo do título
(Conventional Commits):
- feat → pelo menos o número do meio (2.1.3 → 2.2.0);
- fix, docs, build, ci, refactor, perf, test, chore, style → pelo menos o último (2.1.3 → 2.1.4);
- quebra de verdade (o usuário precisa fazer algo à mão ou não dá para voltar à versão anterior) → o primeiro
  (2.1.3 → 3.0.0). Fica a critério de quem faz o PR: feat(banco)! que só acrescenta tabela ou coluna é "feat".

Falha o PR se a versão não subiu ou subiu menos do que o tipo pede.

Uso (na CI): BASE=<branch de destino> TITULO_PR=<título do PR> python3 scripts/ci/versao/verificar-versao.py
"""
import os
import subprocess
import sys

sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), '..'))
from comum import PROPS as ARQUIVO, ler_versao, texto_da_versao as texto, versao_minima, nivel_do_titulo


def versao(conteudo: str) -> tuple[int, int, int]:
    achado = ler_versao(conteudo)
    if not achado:
        print(f'::error title=Versão::Não achei <Version>X.Y.Z</Version> no {ARQUIVO}.')
        sys.exit(1)
    return achado


base = os.environ.get('BASE', 'develop')
titulo = os.environ.get('TITULO_PR', '')

anterior = subprocess.run(['git', 'show', f'origin/{base}:{ARQUIVO}'], capture_output=True, text=True, check=True).stdout
with open(ARQUIVO, encoding='utf-8') as f:
    atual = f.read()
de, para = versao(anterior), versao(atual)
minima = versao_minima(de, titulo)
nome = ('funcionalidade (feat): sobe o número do meio' if nivel_do_titulo(titulo) == 1
        else 'correção ou ajuste: sobe o último número')

if para < minima:
    print(f'::error title=Versão::O {ARQUIVO} está em {texto(para)}; o {base} está em {texto(de)}. '
          f'Este PR é {nome}, então a versão tem de ser pelo menos {texto(minima)} (ADR 0039). '
          f'Se outro PR subiu a versão antes, pegue a maior e suba de novo.')
    sys.exit(1)
print(f'Versão {texto(de)} → {texto(para)} ({nome}).')
