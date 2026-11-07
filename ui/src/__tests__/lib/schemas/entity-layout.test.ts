import {describe, expect, it} from 'vitest';
import {addressFromApi, addressSchema, addressToApi, stateRegistrationSchema} from '@/lib/schemas/entity';
import {LAYOUT_LIMITS} from '@/lib/constants/layout-limits';

const BR = {
  city_ibge_code: '3550308', street: 'Rua A', neighborhood: 'Centro', number: '1', city: 'São Paulo',
  state_federation: 'SP' as const, postal_code: '01001000', complement: '', country_code: '', country_name: '',
};
const ABROAD = {
  ...BR, state_federation: 'EX' as const, city_ibge_code: '9999999', city: 'Lisboa', postal_code: '',
  country_code: '6076', country_name: 'Portugal',
};

const paths = (r: ReturnType<typeof addressSchema.safeParse>) =>
  r.success ? [] : r.error.issues.map((i) => i.path.join('.'));

describe('addressSchema: exterior (AddressBody, validateAddressBody)', () => {
  it('aceita o endereço no exterior sem CEP', () => {
    expect(addressSchema.safeParse(ABROAD).success).toBe(true);
  });

  it('exige o CEP no Brasil', () => {
    expect(paths(addressSchema.safeParse({...BR, postal_code: ''}))).toEqual(['postal_code']);
  });

  it('no exterior exige município 9999999 e um país da tabela diferente do Brasil', () => {
    expect(paths(addressSchema.safeParse({...ABROAD, city_ibge_code: '3550308'}))).toEqual(['city_ibge_code']);
    expect(paths(addressSchema.safeParse({...ABROAD, country_code: ''}))).toEqual(['country_code']);
    expect(paths(addressSchema.safeParse({...ABROAD, country_code: '1058'}))).toEqual(['country_code']);
    expect(paths(addressSchema.safeParse({...ABROAD, country_code: '9998'}))).toEqual(['country_code']);
  });

  it('segue os tamanhos do leiaute', () => {
    const max = LAYOUT_LIMITS.addressText.max;
    expect(addressSchema.safeParse({...BR, street: 'x'.repeat(max)}).success).toBe(true);
    expect(paths(addressSchema.safeParse({...BR, street: 'x'.repeat(max + 1)}))).toEqual(['street']);
    expect(paths(addressSchema.safeParse({...BR, neighborhood: 'x'}))).toEqual(['neighborhood']);
    expect(paths(addressSchema.safeParse({...BR, complement: 'x'.repeat(61)}))).toEqual(['complement']);
  });
});

describe('addressToApi / addressFromApi', () => {
  it('no exterior manda o país e nenhum CEP; no Brasil, o CEP e nenhum país', () => {
    expect(addressToApi(ABROAD)).toMatchObject({postal_code: '', country_code: '6076', country_name: 'Portugal'});
    expect(addressToApi({...BR, postal_code: '01001-000', country_code: '6076'}))
      .toMatchObject({postal_code: '01001000', country_code: null, country_name: null, complement: null});
  });

  it('cadastro antigo sem os campos do exterior volta com strings vazias', () => {
    const legacy = {...BR, complement: null, country_code: undefined, country_name: undefined};
    expect(addressFromApi(legacy)).toMatchObject({country_code: '', country_name: '', complement: ''});
  });
});

describe('stateRegistrationSchema', () => {
  it('aceita a literal ISENTO e recusa fora de 2 a 14', () => {
    expect(stateRegistrationSchema.safeParse({uf: 'SP', state_registration: 'ISENTO'}).success).toBe(true);
    expect(stateRegistrationSchema.safeParse({uf: 'SP', state_registration: '1'}).success).toBe(false);
    expect(stateRegistrationSchema.safeParse({uf: 'SP', state_registration: '1'.repeat(15)}).success).toBe(false);
  });
});
