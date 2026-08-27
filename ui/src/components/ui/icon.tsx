import {
  CalendarClock,
  Container,
  CreditCard,
  ClipboardList,
  Combine,
  FileSignature,
  FileText,
  Percent,
  Receipt,
  Route,
  Truck,
} from 'lucide-react'

interface IconProps {
  width?: number
  height?: number
}

/** NF-e — Nota Fiscal Eletrônica (sales invoice) */
export const NfeIcon = ({width = 16, height = 16}: IconProps) => (
  <FileText width={width} height={height}/>
)

/** NFC-e — Nota Fiscal de Consumidor Eletrônica (POS receipt) */
export const NfceIcon = ({width = 16, height = 16}: IconProps) => (
  <Receipt width={width} height={height}/>
)

/** CT-e — Conhecimento de Transporte Eletrônico (transport doc) */
export const CteIcon = ({width = 16, height = 16}: IconProps) => (
  <Truck width={width} height={height}/>
)

/** MDF-e — Manifesto de Documento Fiscal Eletrônico (transport manifest) */
export const MdfeIcon = ({width = 16, height = 16}: IconProps) => (
  <Route width={width} height={height}/>
)

/** Catálogo de serviços (NFS-e) */
export const ServiceIcon = ({width = 16, height = 16}: IconProps) => (
  <ClipboardList width={width} height={height}/>
)

/** Natureza de operação — o cenário de negócio da emissão */
export const RouteIcon = ({width = 16, height = 16}: IconProps) => (
  <Route width={width} height={height}/>
)

/** Perfil fiscal — tributação reutilizável entre produtos */
export const PercentIcon = ({width = 16, height = 16}: IconProps) => (
  <Percent width={width} height={height}/>
)

/** Condição de pagamento — parcelas e vencimentos reutilizáveis */
export const CalendarClockIcon = ({width = 16, height = 16}: IconProps) => (
  <CalendarClock width={width} height={height}/>
)

/** Terminal de captura (POS) — CNPJ recebedor e id da maquininha */
export const CreditCardIcon = ({width = 16, height = 16}: IconProps) => (
  <CreditCard width={width} height={height}/>
)

/** Unidade de transporte/carga — carreta, vagão, contêiner, pallet */
export const PackageIcon = ({width = 16, height = 16}: IconProps) => (
  <Container width={width} height={height}/>
)

/** Composição veicular — cavalo, reboques e condutores que andam juntos */
export const VehicleSetIcon = ({width = 16, height = 16}: IconProps) => (
  <Combine width={width} height={height}/>
)

/** NFS-e — Nota Fiscal de Serviços Eletrônica */
export const NfseIcon = ({width = 16, height = 16}: IconProps) => (
  <FileSignature width={width} height={height}/>
)

export const ShoppingBagIcon = ({width = 16, height = 16}: IconProps) => (
  <svg width={width} height={height} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"
       strokeLinecap="round" strokeLinejoin="round">
    <path d="M6 2L3 6v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2V6l-3-4z"/>
    <line x1="3" y1="6" x2="21" y2="6"/>
    <path d="M16 10a4 4 0 0 1-8 0"/>
  </svg>
)

export const BriefcaseIcon = ({width = 16, height = 16}: IconProps) => (
  <svg width={width} height={height} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"
       strokeLinecap="round" strokeLinejoin="round">
    <rect x="2" y="7" width="20" height="14" rx="2"/>
    <path d="M16 21V5a2 2 0 0 0-2-2h-4a2 2 0 0 0-2 2v16"/>
  </svg>
)

export const TruckIcon = ({width = 16, height = 16}: IconProps) => (
  <svg width={width} height={height} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"
       strokeLinecap="round" strokeLinejoin="round">
    <rect x="1" y="3" width="15" height="13"/>
    <polygon points="16 8 20 8 23 11 23 16 16 16 16 8"/>
    <circle cx="5.5" cy="18.5" r="2.5"/>
    <circle cx="18.5" cy="18.5" r="2.5"/>
  </svg>
)

export const UsersIcon = ({width = 16, height = 16}: IconProps) => (
  <svg width={width} height={height} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"
       strokeLinecap="round" strokeLinejoin="round">
    <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/>
    <circle cx="9" cy="7" r="4"/>
    <path d="M23 21v-2a4 4 0 0 0-3-3.87"/>
    <path d="M16 3.13a4 4 0 0 1 0 7.75"/>
  </svg>
)

export const ShieldIcon = ({width = 16, height = 16}: IconProps) => (
  <svg width={width} height={height} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"
       strokeLinecap="round" strokeLinejoin="round">
    <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/>
  </svg>
)

export const SettingsIcon = ({width = 16, height = 16}: IconProps) => (
  <svg width={width} height={height} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"
       strokeLinecap="round" strokeLinejoin="round">
    <circle cx="12" cy="12" r="3"/>
    <path
      d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/>
  </svg>
)
