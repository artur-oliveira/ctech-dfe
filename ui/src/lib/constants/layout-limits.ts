/**
 * Leiaute da NF-e na UI: tamanhos de texto e as literais do exterior e do isento.
 *
 * Tamanhos de texto do leiaute da NF-e que o cadastro e o pedido respeitam. Fonte única na UI: schemas Zod,
 * `maxLength` dos inputs e o contador perto do limite leem daqui. Espelham as tags `validate` de
 * `api/internal/api/v1/dto.go` (AddressBody, StateRegistrationBody, ContactsBody, PersonObjectBody, ProductBody) e
 * `NfeLocalBody` (`api/internal/services/nfes/emit.go`), que a api compara com os XSDs em
 * `dto_layout_limits_test.go`. Mudou lá, muda aqui.
 */
export const LAYOUT_LIMITS = {
  /** Razão social / nome (xNome). */
  name: {min: 2, max: 60},
  /** Nome fantasia (xFant). */
  fantasyName: {min: 1, max: 60},
  /** Logradouro, bairro e cidade (xLgr, xBairro, xMun) e nome do país (xPais). */
  addressText: {min: 2, max: 60},
  /** Complemento (xCpl). */
  complement: {min: 1, max: 60},
  /** Número do endereço no cadastro (AddressBody.number). */
  addressNumber: {max: 20},
  /** Número no local de retirada/entrega (TLocal/nro). */
  localNumber: {min: 1, max: 60},
  /** Inscrição estadual (TIe): dígitos ou a literal ISENTO. */
  stateRegistration: {min: 2, max: 14},
  /** E-mail (TEmail no cadastro e no local). */
  email: {min: 6, max: 60},
  /** Descrição do produto (xProd). */
  productDescription: {min: 2, max: 120},
} as const;

/** Inscrição estadual do contribuinte isento: a literal que a api grava e a NF-e traduz em indIEDest 2. */
export const IE_EXEMPT = 'ISENTO';

/** UF do endereço no exterior (e do destino no escopo do CFOP 7xxx) e o município que o leiaute exige com ela. */
export const UF_EXTERIOR = 'EX';
export const C_MUN_EXTERIOR = '9999999';

/** A partir de quanto do limite o contador de caracteres aparece: antes disso ele só ocupa espaço. */
export const LENGTH_HINT_RATIO = 0.8;
