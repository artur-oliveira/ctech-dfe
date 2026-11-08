/** Aviso de que a função da pessoa na organização não cobre a tela; a api também nega (403). */
export function NoPermission({what}: { what: string }) {
  return (
    <div role="alert" className="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900">
      Você não tem permissão para {what}. Peça a um administrador da organização para liberar o acesso.
    </div>
  );
}
