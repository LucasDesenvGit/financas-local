"""Gera os diagramas do banco (erDiagram do Mermaid) de docs/BANCO-DE-DADOS.md a partir de banco/01-estrutura.sql.

Sem argumentos, reescreve o trecho entre os marcadores. Com --conferir, só confere que o desenho está em dia (a CI roda
assim). Cada tabela tem de estar em um assunto de ASSUNTOS: tabela nova sem assunto é erro, de propósito.

Mostra as chaves primárias e estrangeiras; o resto fica no SQL. A seta diz a coluna e o que acontece ao apagar a linha
referenciada. Tabela de outro assunto aparece sem as colunas, só para mostrar a ligação.
"""
import re
import sys

SQL = "banco/01-estrutura.sql"
DOC = "docs/BANCO-DE-DADOS.md"
INICIO = "<!-- diagramas-do-banco: início (gerado por scripts/ci/gerar-diagrama-banco.py) -->"
FIM = "<!-- diagramas-do-banco: fim -->"

ASSUNTOS = {
    "Segurança": ["Usuario", "AuditoriaAcesso", "Configuracao"],
    "Open Finance": ["ConexaoBancaria", "SincronizacaoLog", "CategoriaPluggy"],
    "Financeiro": ["Conta", "FaturaCartao", "Transacao", "Categoria", "RegraCategorizacao", "RegraDescricao"],
    "Planejamento": ["Orcamento", "Meta", "MetaAporte", "ContaAgendada"],
    "Investimentos": ["Investimento", "HistoricoInvestimento", "Ativo", "OperacaoAtivo", "ProventoAtivo", "PrecoMensalAtivo",
                      "Emprestimo", "PagamentoEmprestimo", "ReservaConferencia"],
    "Índices e cotações": ["TaxaSelicDiaria", "IndiceMensal", "CotacaoMoeda", "CotacaoCripto"],
    "Sistema": ["ErroSistema"],
}


def sem_comentarios(texto):
    return re.sub(r"--[^\n]*", "", re.sub(r"/\*.*?\*/", "", texto, flags=re.S))


def ler_tabelas(texto):
    """Devolve {tabela: {"colunas": {nome: (tipo, obrigatoria)}, "pk": [...], "fks": [(coluna, destino, acao)]}}."""
    tabelas = {}
    for nome, corpo in re.findall(r"CREATE TABLE IF NOT EXISTS\s+(\w+)\s*\((.*?)\n\);", sem_comentarios(texto), flags=re.S):
        tabela = {"colunas": {}, "pk": [], "fks": []}
        for linha in corpo.splitlines():
            m = re.match(r"\s*(\w+)\s+(INTEGER|TEXT|REAL|BLOB|NUMERIC)\b(.*)", linha)
            if m and m.group(1).upper() != "CONSTRAINT":
                tabela["colunas"][m.group(1)] = (m.group(2), "NOT NULL" in m.group(3).upper())
                if "PRIMARY KEY" in m.group(3).upper():
                    tabela["pk"].append(m.group(1))
        m = re.search(r"CONSTRAINT\s+\w+\s+PRIMARY KEY\s*\(([^)]*)\)", corpo)
        if m:
            tabela["pk"] += [c.strip() for c in m.group(1).split(",") if c.strip() not in tabela["pk"]]
        for coluna, destino, resto in re.findall(r"FOREIGN KEY\s*\((\w+)\)\s*REFERENCES\s+(\w+)\s*\(\w+\)([^,\n]*(?:\n\s*ON DELETE[^,\n]*)?)", corpo):
            acao = re.search(r"ON DELETE\s+(CASCADE|SET NULL|RESTRICT|NO ACTION)", resto)
            tabela["fks"].append((coluna, destino, acao.group(1) if acao else "bloqueia"))
        tabelas[nome] = tabela
    return tabelas


def diagrama(nomes, tabelas):
    linhas = ["```mermaid", "erDiagram"]
    for nome in nomes:
        t = tabelas[nome]
        fks = [f[0] for f in t["fks"]]
        atributos = [c for c in t["colunas"] if c in t["pk"] or c in fks]
        linhas.append(f"    {nome} {{")
        for c in atributos:
            marca = ",".join(m for m, vale in (("PK", c in t["pk"]), ("FK", c in fks)) if vale)
            linhas.append(f"        {t['colunas'][c][0]} {c} {marca}")
        linhas.append("    }")
    for nome in nomes:
        for coluna, destino, acao in tabelas[nome]["fks"]:
            obrigatoria = tabelas[nome]["colunas"][coluna][1]
            linhas.append(f'    {destino} {"||" if obrigatoria else "|o"}--o{{ {nome} : "{coluna}: {acao}"')
    linhas.append("```")
    return "\n".join(linhas)


def trecho(tabelas):
    partes = [INICIO, ""]
    for assunto, nomes in ASSUNTOS.items():
        partes += [f"### {assunto}", "", diagrama(nomes, tabelas), ""]
    partes.append(FIM)
    return "\n".join(partes)


def principal():
    tabelas = ler_tabelas(open(SQL, encoding="utf-8-sig").read())
    com_assunto = [n for nomes in ASSUNTOS.values() for n in nomes]
    sobrando = sorted(set(tabelas) - set(com_assunto))
    faltando = sorted(set(com_assunto) - set(tabelas))
    if sobrando or faltando:
        print(f"Tabelas sem assunto em ASSUNTOS: {sobrando}; assuntos com tabela que não existe: {faltando}", file=sys.stderr)
        return 1
    bruto = open(DOC, encoding="utf-8", newline="").read()
    crlf = "\r\n" in bruto
    doc = bruto.replace("\r\n", "\n")
    if INICIO not in doc or FIM not in doc:
        print(f"Marcadores não encontrados em {DOC}", file=sys.stderr)
        return 1
    novo = doc[:doc.index(INICIO)] + trecho(tabelas) + doc[doc.index(FIM) + len(FIM):]
    if "--conferir" in sys.argv:
        if novo != doc:
            print(f"{DOC} está velho: rode python3 scripts/ci/gerar-diagrama-banco.py", file=sys.stderr)
            return 1
        print(f"OK: {len(tabelas)} tabelas nos diagramas de {DOC}.")
        return 0
    open(DOC, "w", encoding="utf-8", newline="").write(novo.replace("\n", "\r\n") if crlf else novo)
    print(f"{len(tabelas)} tabelas em {DOC}.")
    return 0


if __name__ == "__main__":
    sys.exit(principal())
