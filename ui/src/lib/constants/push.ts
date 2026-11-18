import type {PushEventType} from '@/lib/types/push';

/** Events a person can turn on, in the order the screen lists them (labels and hints in pt-BR). */
export const PUSH_EVENTS: { value: PushEventType; label: string; hint: string }[] = [
  {value: 'distribution.received', label: 'Nota recebida', hint: 'Chegou um documento emitido contra a sua empresa'},
  {value: 'document.authorized', label: 'Documento autorizado', hint: 'A SEFAZ ou a prefeitura autorizou uma emissão'},
  {value: 'document.rejected', label: 'Documento rejeitado', hint: 'A autoridade recusou uma emissão; o motivo está no documento'},
  {value: 'document.failed', label: 'Falha na emissão', hint: 'O emissor não conseguiu concluir uma emissão'},
];

/** iPhone has no install prompt event: the person adds the site to the Home Screen by hand. */
export const PUSH_IOS_INSTALL_STEPS = [
  'Abra este site no Safari do iPhone.',
  'Toque em Compartilhar, o ícone do quadrado com a seta para cima.',
  'Escolha Adicionar à Tela de Início e confirme.',
  'Abra o CTech DF-e pelo ícone da Tela de Início e volte a esta tela.',
] as const;
