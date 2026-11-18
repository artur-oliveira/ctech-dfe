'use client';

import {CheckboxField} from '@/components/ui/checkbox-field';
import {PUSH_EVENTS} from '@/lib/constants/push';

interface NotificationPreferencesProps {
  /** Event types the installation offers (`config.event_types`); the screen lists only these, in its own order. */
  offered: readonly string[]
  /** Events turned on for the selected company. */
  selected: readonly string[]
  /** A save is in flight. */
  disabled: boolean
  companyName: string
  onChange: (next: string[]) => void
}

/** What to be notified about for the selected company: one checkbox per event, never a free-text field. */
export function NotificationPreferences({offered, selected, disabled, companyName, onChange}: NotificationPreferencesProps) {
  const events = PUSH_EVENTS.filter((e) => offered.includes(e.value));
  const toggle = (value: string, on: boolean) => {
    const rest = selected.filter((v) => v !== value);
    // Keep the screen's order so the saved list does not shuffle between toggles.
    onChange(PUSH_EVENTS.map((e) => e.value).filter((v) => v === value ? on : rest.includes(v)));
  };

  return (
    <section className="rounded-xl border border-gray-200 bg-white p-4 md:p-6" aria-labelledby="push-prefs-title">
      <h2 id="push-prefs-title" className="text-base font-semibold text-gray-900">O que avisar</h2>
      <p className="mt-1 text-sm text-gray-500">
        Vale para <b>{companyName}</b> e para todos os seus dispositivos. Cada pessoa escolhe os seus avisos; trocar de empresa mostra a escolha dela.
      </p>
      <ul className="mt-3 divide-y divide-gray-100">
        {events.map((e) => (
          <li key={e.value} className="py-2">
            <CheckboxField
              id={`push-event-${e.value}`}
              label={e.label}
              checked={selected.includes(e.value)}
              disabled={disabled}
              onCheckedChange={(on) => toggle(e.value, on)}
              className="text-sm font-medium text-gray-900"
            />
            <p className="pl-6 text-xs text-gray-500">{e.hint}</p>
          </li>
        ))}
      </ul>
    </section>
  );
}
