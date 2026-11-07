/**
 * Recusas locais da emissão de NF-e e NFC-e (422 antes da SEFAZ), traduzidas para o que o operador corrige.
 *
 * A api devolve três `code` (`api/internal/services/nfes/prevalidate.go` e `v.go`), cada um com `errors[]`
 * (`problem.FieldError`: `field`, `code`, `message`, `params`):
 * - `invalid_decimal`: número malformado num cadastro; `field` é a chave lida.
 * - `registration_incomplete`: cadastro sem um dado que o leiaute exige; `params.entity` diz qual.
 * - `fiscal_prevalidation`: regra do MOC em block (`fiscal_rule`, `params.rule`/`params.cstat`) ou texto fora do
 *   tamanho do leiaute (`too_long`/`too_short`, `params.entity`/`limit`/`got`/`item`/`xml_path`).
 *
 * O texto é montado aqui, em pt-BR, a partir dos códigos e dos números; o `message` da api só serve para a
 * descrição da regra do MOC, que o catálogo do go-dfe já escreve em português.
 */

import {extractId, SK_PREFIX} from '@/lib/constants/entity-keys';
import {unformatCpfCnpj} from '@/lib/utils/document';

export const PROBLEM_INVALID_DECIMAL = 'invalid_decimal';
export const PROBLEM_REGISTRATION_INCOMPLETE = 'registration_incomplete';
export const PROBLEM_FISCAL_PREVALIDATION = 'fiscal_prevalidation';

const FIELD_TOO_LONG = 'too_long';
const FIELD_TOO_SHORT = 'too_short';
const FIELD_FISCAL_RULE = 'fiscal_rule';

const EMIT_PROBLEM_CODES = new Set([PROBLEM_INVALID_DECIMAL, PROBLEM_REGISTRATION_INCOMPLETE, PROBLEM_FISCAL_PREVALIDATION]);

/**
 * Onde o operador corrige: um cadastro (emitente, destinatário, produto, transportadora) ou um bloco do próprio
 * formulário (locais, finalidade, itens, pagamento). As cinco primeiras são as `params.entity` da api.
 */
export type EmitTarget =
  | 'emitente' | 'destinatario' | 'produto' | 'transportadora' | 'documento'
  | 'retirada' | 'entrega' | 'finalidade' | 'itens' | 'pagamento';

const TARGET_LABELS: Record<EmitTarget, string> = {
  emitente: 'Emitente (sua empresa)',
  destinatario: 'Destinatário',
  produto: 'Produto',
  transportadora: 'Transportadora',
  documento: 'Nota',
  retirada: 'Local de retirada',
  entrega: 'Local de entrega',
  finalidade: 'Finalidade',
  itens: 'Itens',
  pagamento: 'Pagamento',
};

/** Rótulo de cada campo pelo último segmento do caminho que a api devolve (cadastro ou NfeLocalBody). */
const FIELD_LABELS: Record<string, string> = {
  name: 'Nome', fantasy_name: 'Nome fantasia', state_registration: 'Inscrição estadual', emails: 'E-mail',
  phones: 'Telefone', street: 'Logradouro', number: 'Número', complement: 'Complemento',
  neighborhood: 'Bairro', city: 'Cidade', state_federation: 'UF', city_ibge_code: 'Município (código IBGE)',
  country_code: 'País', country_name: 'Nome do país', description: 'Descrição', code: 'Código',
  inf_ad_prod: 'Informações adicionais do item', issqn_c_mun_fg: 'Município do fato gerador do ISSQN',
  x_nome: 'Nome', x_lgr: 'Logradouro', nro: 'Número', x_cpl: 'Complemento', x_bairro: 'Bairro',
  x_mun: 'Cidade', x_pais: 'Nome do país', c_pais: 'País', email: 'E-mail', ie: 'Inscrição estadual',
};

/**
 * Regras do MOC em block, por família (`blockRules` em prevalidate.go): onde corrigir e como. A chave é o prefixo
 * do id da regra; a primeira que casar vence, então as mais específicas vêm antes.
 */
const RULE_HINTS: { prefix: string; target: EmitTarget; hint: string }[] = [
  {prefix: 'F07', target: 'retirada', hint: 'No exterior o local de retirada leva UF EX, o país da tabela e o município 9999999.'},
  {prefix: 'G07', target: 'entrega', hint: 'No exterior o local de entrega leva UF EX, o país da tabela e o município 9999999.'},
  {prefix: 'E16a-30', target: 'destinatario', hint: 'A UF do destinatário não aceita contribuinte isento: informe o número da inscrição estadual ou retire a inscrição do cadastro.'},
  {prefix: 'E17-30', target: 'destinatario', hint: 'A inscrição ISENTO não sai na nota: confira a inscrição estadual do destinatário.'},
  {prefix: 'E', target: 'destinatario', hint: 'Destinatário no exterior: o endereço leva UF EX, município 9999999, o país da tabela e nenhuma inscrição estadual.'},
  {prefix: 'I08', target: 'itens', hint: 'O CFOP não combina com o destino: 5xxx dentro do estado, 6xxx para outro estado, 7xxx para o exterior. Ajuste a natureza da operação ou o CFOP do produto.'},
  {prefix: 'B25', target: 'finalidade', hint: 'Nota de crédito é sempre de entrada e nota de débito, de saída; cada uma exige o seu tipo, e os itens só levam IBS e CBS (salvo os tipos que a regra permite).'},
  {prefix: 'YA', target: 'pagamento', hint: 'A soma dos pagamentos precisa fechar com o total da nota; o que passar do total é troco, e "sem pagamento" não leva valor.'},
];

export interface EmitIssue {
  /** O que está errado, em uma frase. */
  text: string
  /** Como corrigir, quando a frase sozinha não diz. */
  hint?: string
  target: EmitTarget
  /** Item da nota (base 1), quando o problema é de um item. */
  item?: number
  /** Campo como a api nomeou (`entrega.c_pais`, `person.addresses[0].street`, id da regra). */
  field: string
}

interface ApiFieldError {
  field?: string
  code?: string
  message?: string
  params?: Record<string, unknown>
}

function fieldLabel(field: string): string {
  const last = field.replace(/\[\d+\]/g, '').split('.').pop() ?? field;
  return FIELD_LABELS[last] ?? field;
}

function targetLabel(target: EmitTarget, item?: number): string {
  return item ? `${TARGET_LABELS[target]} do item ${item}` : TARGET_LABELS[target];
}

function asTarget(entity: unknown): EmitTarget {
  return typeof entity === 'string' && entity in TARGET_LABELS ? entity as EmitTarget : 'documento';
}

const plural = (n: number, one: string, many: string) => `${n} ${n === 1 ? one : many}`;

function issueFrom(problemCode: string, e: ApiFieldError): EmitIssue {
  const field = e.field ?? '';
  const p = e.params ?? {};
  const item = typeof p.item === 'number' && p.item > 0 ? p.item : undefined;

  if (e.code === FIELD_FISCAL_RULE) {
    const rule = String(p.rule ?? field);
    const family = RULE_HINTS.find((r) => rule.startsWith(r.prefix));
    // "regra X do MOC (rejeição N): <descrição>": a descrição é o que interessa ao operador.
    const description = (e.message ?? '').split('): ').slice(1).join('): ');
    return {
      text: `Regra ${rule} do MOC; a SEFAZ rejeitaria a nota com o código ${String(p.cstat ?? '')}${description ? `: ${description}` : ''}.`,
      hint: family?.hint,
      target: family?.target ?? 'documento',
      field,
    };
  }

  const target = asTarget(p.entity);
  const where = targetLabel(target, item);
  const label = fieldLabel(field);
  const limit = Number(p.limit);
  const got = Number(p.got);

  if (e.code === FIELD_TOO_LONG && limit && got) {
    return {
      text: `${where}: ${label} tem ${got} caracteres e o leiaute aceita até ${limit}.`,
      hint: `Corte ${plural(got - limit, 'caractere', 'caracteres')}.`,
      target, item, field,
    };
  }
  if (e.code === FIELD_TOO_SHORT && limit) {
    return {
      text: `${where}: ${label} tem ${plural(got || 0, 'caractere', 'caracteres')} e o leiaute pede ao menos ${limit}.`,
      hint: `Complete com mais ${plural(limit - (got || 0), 'caractere', 'caracteres')}.`,
      target, item, field,
    };
  }
  if (problemCode === PROBLEM_INVALID_DECIMAL) {
    return {
      text: `Número malformado em ${field || 'um valor'}.`,
      hint: 'Corrija o valor no cadastro de onde ele vem (produto, perfil de tributação ou operação) e emita de novo.',
      target, field,
    };
  }
  return {text: `${where}: falta ${label}.`, target, item, field};
}

/** As recusas locais da emissão, ou null quando o erro é outro (a mensagem da api basta). */
export function parseEmitIssues(raw: unknown): EmitIssue[] | null {
  const body = raw as {code?: unknown; errors?: unknown} | undefined;
  if (typeof body?.code !== 'string' || !EMIT_PROBLEM_CODES.has(body.code) || !Array.isArray(body.errors)) return null;
  const issues = (body.errors as ApiFieldError[]).map((e) => issueFrom(body.code as string, e));
  return issues.length > 0 ? issues : null;
}

/** Erros de um local de retirada/entrega por campo do NfeLocalBody (`x_lgr`, `c_pais`...), para mostrar no campo. */
export function localFieldErrors(issues: EmitIssue[] | undefined, target: 'retirada' | 'entrega'): Record<string, string> {
  const out: Record<string, string> = {};
  for (const i of issues ?? []) {
    if (i.target === target && i.field.startsWith(`${target}.`)) out[i.field.slice(target.length + 1)] = i.text;
  }
  return out;
}

/** Telas de edição dos cadastros que uma recusa aponta. */
export const RECORD_HREF = {
  organization: (pk: string) => `/organizations/edit?pk=${encodeURIComponent(pk)}`,
  person: (sk: string) => `/persons/edit?id=${unformatCpfCnpj(sk)}`,
  product: (sk: string) => `/products/edit?id=${extractId(sk, SK_PREFIX.PRODUCT)}`,
};
