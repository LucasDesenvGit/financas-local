package carteira

import (
	"context"
	"database/sql"
	"fmt"
)

// chaveTokenBrapi é a linha do token da brapi na Configuracao, a mesma do .NET.
const chaveTokenBrapi = "Cotacao.BrapiToken"

// ApagarTokenDaBrapi apaga o token da brapi que ficou guardado: a fonte saiu na #763 (ADR 0072), e o token virou um
// segredo sem uso. Roda a cada partida (internal/partida) e devolve quantas linhas apagou: 1 na primeira vez, se havia
// token, e 0 daí em diante.
func ApagarTokenDaBrapi(ctx context.Context, tx *sql.Tx) (int, error) {
	res, err := tx.ExecContext(ctx, `DELETE FROM "Configuracao" WHERE "Chave" = ?`, chaveTokenBrapi)
	if err != nil {
		return 0, fmt.Errorf("apagar o token da brapi: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("apagar o token da brapi: %w", err)
	}
	return int(n), nil
}
