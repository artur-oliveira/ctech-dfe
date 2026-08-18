/**
 * Dev mock API entry point, importado pelo root layout.
 *
 * O adapter axios NÃO é anexado aqui: o layout é server component, então este
 * módulo só é avaliado no servidor e o `setAdapter` nunca chegaria ao browser.
 * Quem anexa é `MockDevPanel` — módulo cliente, avaliado na hidratação, antes
 * de qualquer efeito de provider rodar.
 */

import {MOCK_ENABLED} from './env'
import {MockDevPanel} from './MockDevPanel'

export {MockDevPanel, MOCK_ENABLED}
