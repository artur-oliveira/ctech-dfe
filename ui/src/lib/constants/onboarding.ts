import type {DocVariant} from '@/lib/schemas/fiscal-configs'

/**
 * The first-run flow, in layers.
 *
 * Onboarding is ordered because setup genuinely is: there is no company to
 * configure before a plan grants one, no document numbering before a company,
 * and no product catalogue before a document type that consumes it. The numbers
 * carry that dependency — they are not decoration.
 *
 * The last two layers are conditional: a carrier that only moves freight never
 * sees the product step, and a clinic never sees it either. Asking everyone
 * everything is how a setup flow turns into a form to be endured.
 */

export const ONBOARDING_ROOT = '/onboarding'

export const STEP_PLAN = 'plano'
export const STEP_COMPANY = 'empresa'
export const STEP_DOCUMENTS = 'documentos'
export const STEP_PRODUCTS = 'produtos'
export const STEP_SERVICES = 'servicos'
export const STEP_DONE = 'pronto'

/** The checkout return lives under the flow but is not a step of its own. */
export const STEP_CHECKOUT_RETURN = 'retorno'

export type OnboardingStep =
  | typeof STEP_PLAN
  | typeof STEP_COMPANY
  | typeof STEP_DOCUMENTS
  | typeof STEP_PRODUCTS
  | typeof STEP_SERVICES
  | typeof STEP_DONE

export interface StepDefinition {
  id: OnboardingStep
  /** Shown in the rail. Kept to one or two words so it survives 375px. */
  label: string
  /** The promise of the step, in the user's words. */
  title: string
  path: string
}

export const ONBOARDING_STEPS: StepDefinition[] = [
  {
    id: STEP_PLAN,
    label: 'Plano',
    title: 'Escolha seu plano',
    path: `${ONBOARDING_ROOT}/${STEP_PLAN}`,
  },
  {
    id: STEP_COMPANY,
    label: 'Empresa',
    title: 'Cadastre sua empresa',
    path: `${ONBOARDING_ROOT}/${STEP_COMPANY}`,
  },
  {
    id: STEP_DOCUMENTS,
    label: 'Documentos',
    title: 'O que você emite',
    path: `${ONBOARDING_ROOT}/${STEP_DOCUMENTS}`,
  },
  {
    id: STEP_PRODUCTS,
    label: 'Produtos',
    title: 'Seus produtos',
    path: `${ONBOARDING_ROOT}/${STEP_PRODUCTS}`,
  },
  {
    id: STEP_SERVICES,
    label: 'Serviços',
    title: 'Seus serviços',
    path: `${ONBOARDING_ROOT}/${STEP_SERVICES}`,
  },
  {
    id: STEP_DONE,
    label: 'Pronto',
    title: 'Tudo pronto',
    path: `${ONBOARDING_ROOT}/${STEP_DONE}`,
  },
]

/** Document types whose issuance consumes the product catalogue. */
export const PRODUCT_DOC_VARIANTS: DocVariant[] = ['nfe', 'nfce']

/** Document types whose issuance consumes the service catalogue. */
export const SERVICE_DOC_VARIANTS: DocVariant[] = ['nfse']

/**
 * Document types that need NF-e configured even when the company never issues
 * one.
 *
 * A CT-e is written against the NF-e of the cargo it carries, and an MDF-e lists
 * them; both pull those notes from NF-e distribution, which only runs for an
 * organization that has an `nfe_config` (see `worker/internal/service/
 * distribution.go`). So the flow creates one silently, with numbering at zero —
 * a configuration that receives and never issues.
 */
export const DISTRIBUTION_DEPENDENT_VARIANTS: DocVariant[] = ['cte', 'mdfe']

/** The variant that dependency creates. */
export const DISTRIBUTION_SOURCE_VARIANT: DocVariant = 'nfe'

/**
 * Where "issue your first one" goes, per document type.
 *
 * CT-e points at its list rather than an issuance screen because that screen
 * does not exist yet — sending someone to a 404 is a worse ending than sending
 * them to the place the feature will appear.
 */
export const FIRST_ISSUANCE_PATH: Record<DocVariant, string> = {
  nfe: '/nfe/emit',
  nfce: '/nfce/emit',
  cte: '/cte',
  mdfe: '/mdfe/emit',
  nfse: '/nfse/emit',
}

