// Pacote sessao diz quem está logado na requisição. Quem confere o login é o internal/plataforma/autenticacao (#75), que põe o
// Id do usuário no contexto antes de chamar a rota.
package sessao

import "context"

type chaveUsuario struct{}

// ComUsuario devolve um contexto com o Id do usuário logado.
func ComUsuario(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, chaveUsuario{}, id)
}

// Usuario é o Id do usuário logado, e false se a requisição não tem um (rota anônima, ou não passou pela autenticação).
func Usuario(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(chaveUsuario{}).(int64)
	return id, ok && id > 0
}
