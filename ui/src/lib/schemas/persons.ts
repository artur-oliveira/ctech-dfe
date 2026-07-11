/**
 * Person schema — re-exports from the unified entity schema.
 */
export {
  entitySchema as personSchema,
  type EntityFormData as PersonFormData,
  addressSchema as personAddressSchema,
  stateRegistrationSchema,
  UF_OPTIONS,
} from '@/lib/schemas/entity'

export type {AddressData as PersonAddressData, StateRegistrationData} from '@/lib/schemas/entity'
