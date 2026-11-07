import {z} from 'zod';
import {validateCNPJ, validateCPF, cleanDocument} from '@/lib/utils/validators';

export const MAX_AUTHORIZED_VIEWERS = 10;

export const authorizedViewerSchema = z.object({
  cpf_or_cnpj: z.string().min(1, 'CPF/CNPJ obrigatório'),
  name: z.string().min(2, 'Mínimo 2 caracteres').max(60),
}).superRefine((data, ctx) => {
  const raw = cleanDocument(data.cpf_or_cnpj);
  const valid = raw.length === 14 ? validateCNPJ(raw) : validateCPF(raw);
  if (!valid) {
    ctx.addIssue({code: 'custom', message: 'CPF/CNPJ inválido', path: ['cpf_or_cnpj']});
  }
});

export type AuthorizedViewerFormData = z.infer<typeof authorizedViewerSchema>

/** Client-side pre-check mirroring the backend's dedup rule (409 on repeat),
 * so the user gets inline feedback instead of a round-trip failure. */
export function hasDuplicateViewer(existing: {cpf_cnpj: string}[], cpfOrCnpj: string): boolean {
  const raw = cleanDocument(cpfOrCnpj);
  return existing.some((v) => cleanDocument(v.cpf_cnpj) === raw);
}
