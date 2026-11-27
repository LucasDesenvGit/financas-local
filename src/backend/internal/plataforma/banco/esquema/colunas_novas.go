package esquema

// ColunasNovas são as colunas acrescentadas a tabelas que já existiam (ADR 0003). Toda coluna daqui também está no
// CREATE TABLE do banco/01-estrutura.sql: o CREATE TABLE cria o banco novo, e o AplicarEsquema acrescenta ao banco antigo
// as que faltam (ALTER TABLE ... ADD COLUMN, só se a coluna não existe). O TestColunasNovasTambemNoCreateTable confere.
//
// Coluna nova: uma linha no fim e a mesma definição no CREATE TABLE. A lista entra na assinatura do banco
// (PRAGMA user_version): não mude a ordem nem o texto das que já existem, ou todo banco antigo ganha uma cópia de
// segurança de novo. Os nomes só aceitam letras, números e "_": vão para o SQL sem parâmetro, e só vêm daqui.
var ColunasNovas = []ColunaNova{
	{"Ativo", "Moeda", "TEXT NOT NULL DEFAULT 'BRL'"},
	{"OperacaoAtivo", "Cambio", "TEXT NOT NULL DEFAULT '1'"},
	{"Meta", "ReservaNome", "TEXT NULL"},
	{"Meta", "Prioridade", "INTEGER NOT NULL DEFAULT 0"},
	{"ContaAgendada", "DataLimite", "TEXT NULL"},
	{"Transacao", "DescricaoRevisada", "INTEGER NOT NULL DEFAULT 0"},
	{"Conta", "Banco", "TEXT NULL"},
	{"Conta", "ContaVinculadaId", "INTEGER NULL"},
	{"Conta", "SaldoReservado", "TEXT NULL"},
	{"Conta", "ReservasInformadas", "TEXT NULL"},
	{"Transacao", "MeioPagamento", "TEXT NULL"},
	{"Transacao", "MesmaTitularidade", "INTEGER NULL"},
	{"Transacao", "RegraDescricaoId", "INTEGER NULL"},
	{"Transacao", "DescricaoManual", "INTEGER NOT NULL DEFAULT 0"},
	{"Investimento", "NomePersonalizado", "TEXT NULL"},
	{"Conta", "LimiteAjustadoNoApp", "INTEGER NULL"},
	{"Conta", "LimiteDiferencaAvaliada", "TEXT NULL"},
	{"ContaAgendada", "FormaCobranca", "INTEGER NOT NULL DEFAULT 1"},
	{"ContaAgendada", "TextoCobranca", "TEXT NULL"},
	{"ContaAgendada", "Observacao", "TEXT NULL"},
	{"ContaAgendada", "ChaveExtrato", "TEXT NULL"},
	{"RegraDescricao", "ValorMinimo", "TEXT NULL"},
	{"RegraDescricao", "ValorMaximo", "TEXT NULL"},
	{"Meta", "Icone", "TEXT NULL"},
	{"Meta", "GuardarPorMes", "TEXT NULL"},
	{"ContaAgendada", "ReservaNome", "TEXT NULL"},
	{"ContaAgendada", "ReservaDesde", "TEXT NULL"},
	{"Transacao", "ReservaNome", "TEXT NULL"},
	{"Holerite", "Origem", "INTEGER NOT NULL DEFAULT 1"},
	{"Emprego", "AdiantamentoQuinzenal", "INTEGER NOT NULL DEFAULT 0"},
}
