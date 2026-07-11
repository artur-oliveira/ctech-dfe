/**
 * Organization schema — re-exports from the unified entity schema.
 * Organizations use all fields; persons omit description and contacts.
 */
export {
  type EntityFormData as OrganizationFormData,
  addressSchema,
  stateRegistrationSchema,
  UF_OPTIONS,
} from '@/lib/schemas/entity'
