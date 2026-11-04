// Pacote sessao diz quem está logado na requisição. Quem confere o login é o .NET, que fica na frente (ADR 0049):
// ele manda o Id do usuário num cabeçalho interno, e o servidor do Go o põe no contexto da requisição.
package sessao

import "context"

type chaveUsuario struct{}

// ComUsuario devolve um contexto com o Id do usuário logado.
func ComUsuario(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, chaveUsuario{}, id)
}

// Usuario é o Id do usuário logado, e false se a requisição não tem um (não passou pelo .NET).
func Usuario(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(chaveUsuario{}).(int64)
	return id, ok && id > 0
}
