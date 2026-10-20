'use client';

import {useEffect} from 'react';
import {ErrorState} from '@aoctech/ui';
import {Button} from '@/components/ui/button';

/**
 * The last resort: an error in the root layout or the provider tree, where the
 * route boundary itself never mounted. It has to render its own <html> and
 * <body>, because the ones in the layout are what failed.
 *
 * Fonts and the theme come from that layout, so this screen renders plainer
 * than the rest of the product. That is the point — it has to work when
 * nothing else did.
 */
export default function GlobalError({error, reset}: { error: Error & {digest?: string}; reset: () => void }) {
  useEffect(() => {
    console.error(error);
  }, [error]);

  return (
    <html lang="pt-BR">
      <body>
        <ErrorState
          status={500}
          locale="pt-BR"
          className="min-h-screen justify-center"
          title="O aplicativo não conseguiu iniciar"
          description={`A tela foi interrompida com segurança. Nenhum documento foi emitido ou alterado. ${
            error.digest ? `Referência do erro: ${error.digest}` : 'Tente abrir o aplicativo novamente.'
          }`}
          action={<Button variant="brand" onClick={reset}>Abrir novamente</Button>}
        />
      </body>
    </html>
  );
}
