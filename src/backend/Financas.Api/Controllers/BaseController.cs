using Financas.Api.Dtos;
using Microsoft.AspNetCore.Mvc;

namespace Financas.Api.Controllers;

[ApiController]
[Produces("application/json")]
public abstract class BaseController : ControllerBase
{
    protected ObjectResult Erro(string mensagem, int status = StatusCodes.Status400BadRequest)
        => StatusCode(status, new MensagemDto(mensagem));

    protected ObjectResult NaoEncontrado(string mensagem = "Registro não encontrado.")
        => Erro(mensagem, StatusCodes.Status404NotFound);
}
