'use client';

import {useEffect, useId, useRef, useState} from 'react';
import {Button} from '@aoctech/ui';

export interface EventsMenuItem {
  key: string
  label: string
  hint?: string
  onSelect: () => void
}
export interface EventsMenuGroup {
  label: string
  items: EventsMenuItem[]
  /** Aviso fixo abaixo do título do grupo. */
  note?: string
}

/** Um único ponto de entrada para os eventos fiscais: botão que abre a lista agrupada. */
export function EventsMenu({label = 'Eventos fiscais', groups, className}: { label?: string; groups: EventsMenuGroup[]; className?: string }) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const panelId = useId();

  useEffect(() => {
    if (!open) return undefined;
    const away = (e: MouseEvent) => { if (!ref.current?.contains(e.target as Node)) setOpen(false); };
    const esc = (e: KeyboardEvent) => { if (e.key === 'Escape') setOpen(false); };
    document.addEventListener('mousedown', away);
    document.addEventListener('keydown', esc);
    return () => {
      document.removeEventListener('mousedown', away);
      document.removeEventListener('keydown', esc);
    };
  }, [open]);

  const visible = groups.filter((g) => g.items.length > 0);
  if (visible.length === 0) return null;

  return (
    <div ref={ref} className="relative">
      <Button type="button" variant="outline" aria-expanded={open} aria-controls={panelId}
              onClick={() => setOpen((o) => !o)} className={className ?? 'min-h-11 sm:min-h-0'}>
        {label}
      </Button>
      {open && (
        <div id={panelId} role="group" aria-label={label}
             className="absolute right-0 z-30 mt-1 max-h-[70vh] w-[min(20rem,calc(100vw-2rem))] overflow-y-auto rounded-xl border border-gray-200 bg-white p-2 shadow-lg">
          {visible.map((g) => (
            <div key={g.label} className="py-1">
              <p className="px-2 pb-1 text-xs font-semibold text-gray-500">{g.label}</p>
              {g.note && <p className="px-2 pb-1 text-xs text-gray-500">{g.note}</p>}
              {g.items.map((it) => (
                <button key={it.key} type="button"
                        onClick={() => { setOpen(false); it.onSelect(); }}
                        className="flex min-h-11 w-full flex-col justify-center rounded-lg px-2 text-left text-sm text-gray-800 hover:bg-gray-50 focus-visible:outline-2 focus-visible:outline-brand-500">
                  <span>{it.label}</span>
                  {it.hint && <span className="text-xs text-gray-500">{it.hint}</span>}
                </button>
              ))}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
